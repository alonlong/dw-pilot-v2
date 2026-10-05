// Package tenant 建立、重建、删除预览槽位租户：一个按快照导入数据、写好测试账号的 schema。
//
// 契约与 abby-pm 的 internal/tenant 相同。**只处理槽位**：名字必须是 dlpv 加数字，
// 任何操作都先校验，没有开关可以关掉——删 schema 是本应用唯一不可撤销的操作，
// 不能让一个手误的名字删掉别的 schema。唯一的例外是本地开发租户 dev（DevTenant），
// 它的名字是代码里的常量，不来自输入，见 SeedDev、ResetDev。
package tenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/dailylog/api/internal/auth"
	"example.com/dailylog/api/internal/db"
)

// SlotPrefix 是预览槽位租户名的前缀。
const SlotPrefix = "dlpv"

// DevTenant 是本地开发用的租户。
const DevTenant = "dev"

var reSlot = regexp.MustCompile(`^` + SlotPrefix + `[0-9]{1,3}$`)

// IsSlot 报告 name 是否是预览槽位：前缀加 1～3 位数字，前后没有别的字符。
// 数字是必须的：只有前缀会放过 dlpvault 这类可能是真实租户的名字。
func IsSlot(name string) bool { return reSlot.MatchString(name) }

func requireSlot(name string) error {
	if !IsSlot(name) {
		return fmt.Errorf("租户 %q 不是预览槽位（%s 加数字）——这个命令会删除并重建 schema，只对槽位执行", name, SlotPrefix)
	}
	return nil
}

// ErrNoPassword 表示没有提供测试账号的密码（TENANT_TEST_PASSWORD）。
var ErrNoPassword = errors.New("没有测试账号密码：设置环境变量 TENANT_TEST_PASSWORD")

// Runner 保存各操作共用的设置。
type Runner struct {
	// DSN 是 Postgres 连接串。
	DSN string
	// Out 接收进度输出。
	Out io.Writer
	// Password 是全部测试账号的密码。
	Password string
}

// Seed 建立一个还不存在的槽位：schema、迁移、快照数据、测试账号。已存在时拒绝
// （往已有数据上导入要么中途冲突，要么得到一个既不是快照也不是原样的租户），重建用 Reset。
func (r Runner) Seed(ctx context.Context, name string, snap *Snapshot, accts *Accounts) error {
	if err := requireSlot(name); err != nil {
		return err
	}
	return r.seedNew(ctx, name, snap, accts)
}

// Reset 删除槽位（如存在）再重新建立。重复执行得到相同的数据：日报等行来自快照，
// 账号的邮箱、名字、角色来自账号文件（用户 id 由数据库分配，每次不同）。
// 槽位里的一切都会被清掉，包括测试账号的会话。
func (r Runner) Reset(ctx context.Context, name string, snap *Snapshot, accts *Accounts) error {
	if err := requireSlot(name); err != nil {
		return err
	}
	return r.reset(ctx, name, snap, accts)
}

// Drop 删除槽位：schema 及其中一切、租户登记、该租户的用户和会话。槽位不存在不算错误。
func (r Runner) Drop(ctx context.Context, name string) error {
	if err := requireSlot(name); err != nil {
		return err
	}
	return r.locked(ctx, name, func(pool *pgxpool.Pool) error {
		return r.drop(ctx, pool, name)
	})
}

// SeedDev 建立本地开发租户 dev；已存在时拒绝。
func (r Runner) SeedDev(ctx context.Context, snap *Snapshot, accts *Accounts) error {
	return r.seedNew(ctx, DevTenant, snap, accts)
}

// ResetDev 删除并重建本地开发租户 dev。只会删除名为 dev 的 schema。
func (r Runner) ResetDev(ctx context.Context, snap *Snapshot, accts *Accounts) error {
	return r.reset(ctx, DevTenant, snap, accts)
}

func (r Runner) seedNew(ctx context.Context, name string, snap *Snapshot, accts *Accounts) error {
	if err := r.precheck(name, snap, accts); err != nil {
		return err
	}
	return r.locked(ctx, name, func(pool *pgxpool.Pool) error {
		exists, err := tenantExists(ctx, pool, name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("租户 %s 已存在——重建请用 reset（dev 租户用 devseed -reset）", name)
		}
		return r.seed(ctx, pool, name, snap, accts)
	})
}

func (r Runner) reset(ctx context.Context, name string, snap *Snapshot, accts *Accounts) error {
	if err := r.precheck(name, snap, accts); err != nil {
		return err
	}
	return r.locked(ctx, name, func(pool *pgxpool.Pool) error {
		if err := r.drop(ctx, pool, name); err != nil {
			return err
		}
		return r.seed(ctx, pool, name, snap, accts)
	})
}

// precheck 在连接数据库之前检查参数。
func (r Runner) precheck(name string, snap *Snapshot, accts *Accounts) error {
	if r.Password == "" {
		return ErrNoPassword
	}
	want, err := db.TenantTarget()
	if err != nil {
		return err
	}
	if snap.Manifest.TenantMigration != want {
		return fmt.Errorf("快照 %q 是在租户迁移 %d 时切出的，代码在 %d——请在 %d 的 schema 上重新切快照",
			snap.Manifest.Name, snap.Manifest.TenantMigration, want, want)
	}
	if accts.Tenant != name {
		return fmt.Errorf("账号属于租户 %q，不是 %q", accts.Tenant, name)
	}
	return accts.validate()
}

// locked 先跑 public 迁移，再持有以租户名为键的 advisory lock 执行 fn：
// 平台重试时两次重建同一个槽位会依次执行，而不是互相删掉对方建了一半的 schema。
func (r Runner) locked(ctx context.Context, name string, fn func(*pgxpool.Pool) error) error {
	if _, err := db.QuoteTenant(name); err != nil {
		return err
	}
	if err := db.MigratePublic(ctx, r.DSN); err != nil {
		return err
	}
	pool, err := db.Open(ctx, r.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("连接 Postgres：%w", err)
	}
	defer conn.Release()
	key := "dl-tenant:" + name
	if _, err := conn.Exec(ctx, `select pg_advisory_lock(hashtext($1))`, key); err != nil {
		return fmt.Errorf("锁定 %s：%w", name, err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `select pg_advisory_unlock(hashtext($1))`, key)
	}()
	return fn(pool)
}

func (r Runner) drop(ctx context.Context, pool *pgxpool.Pool, name string) error {
	ident, err := db.QuoteTenant(name)
	if err != nil {
		return err
	}
	exists, err := tenantExists(ctx, pool, name)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Fprintf(r.Out, "%s：不存在，无需删除\n", name)
		return nil
	}
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `drop schema if exists `+ident+` cascade`); err != nil {
			return fmt.Errorf("删除 schema %s：%w", name, err)
		}
		// 用户与会话随租户登记级联删除。
		if _, err := tx.Exec(ctx, `delete from public.tenant where name = $1`, name); err != nil {
			return fmt.Errorf("删除租户登记 %s：%w", name, err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "%s：已删除\n", name)
	return nil
}

func (r Runner) seed(ctx context.Context, pool *pgxpool.Pool, name string, snap *Snapshot, accts *Accounts) error {
	ident, err := db.QuoteTenant(name)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, `create schema `+ident); err != nil {
		return fmt.Errorf("建立 schema %s：%w", name, err)
	}
	// 从这里开始建了东西，失败就删掉：迁移过但没有数据的租户会被下次 seed 拒绝，
	// 登录进去的人还会以为它已经准备好了。
	err = db.MigrateTenant(ctx, r.DSN, name)
	if err == nil {
		err = db.InTenant(ctx, pool, name, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `insert into public.tenant (name, display_name) values ($1, $2)`, name, displayName(name)); err != nil {
				return fmt.Errorf("登记租户 %s：%w", name, err)
			}
			if err := r.load(ctx, tx, name, snap); err != nil {
				return err
			}
			return r.provision(ctx, tx, name, accts)
		})
	}
	if err != nil {
		if dropErr := r.drop(ctx, pool, name); dropErr != nil {
			return fmt.Errorf("%w（删除建了一半的租户也失败了：%v）", err, dropErr)
		}
		return err
	}
	fmt.Fprintf(r.Out, "%s：已按快照 %q 建立\n", name, snap.Manifest.Name)
	return nil
}

func displayName(name string) string {
	if name == DevTenant {
		return "本地开发"
	}
	return "预览槽位 " + name
}

// column 是导入需要知道的目标表列信息。
type column struct {
	Generated bool
	Serial    bool
}

func tableColumns(ctx context.Context, tx pgx.Tx, schema, table string) (map[string]column, error) {
	rows, err := tx.Query(ctx, `select column_name, is_generated = 'ALWAYS',
		  (coalesce(column_default, '') like 'nextval(%' or is_identity = 'YES')
		from information_schema.columns
		where table_schema = $1 and table_name = $2`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]column{}
	for rows.Next() {
		var n string
		var c column
		if err := rows.Scan(&n, &c.Generated, &c.Serial); err != nil {
			return nil, err
		}
		out[n] = c
	}
	return out, rows.Err()
}

// batchRows 是一条语句写入的行数。
const batchRows = 500

// load 按 manifest 的顺序逐表写入快照的行。
//
// 每行只写它列出的列，没列出的取默认值；列出不同列的行分成不同语句写入。
// 值由 Postgres 自己转换（jsonb_populate_recordset 按表的行类型），日期、时间的解析
// 规则只有数据库一套。
func (r Runner) load(ctx context.Context, tx pgx.Tx, schema string, snap *Snapshot) error {
	for i, t := range snap.Manifest.Tables {
		cols, err := tableColumns(ctx, tx, schema, t.Name)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			return fmt.Errorf("schema 里没有表 %s", t.Name)
		}
		groups, err := groupRows(t.Name, snap.rows[i], cols)
		if err != nil {
			return err
		}
		qualified := pgx.Identifier{schema, t.Name}.Sanitize()
		var before int64
		if err := tx.QueryRow(ctx, `select count(*) from `+qualified).Scan(&before); err != nil {
			return err
		}
		serial := map[string]bool{}
		for _, grp := range groups {
			list := make([]string, len(grp.names))
			for j, n := range grp.names {
				list[j] = pgx.Identifier{n}.Sanitize()
				if cols[n].Serial {
					serial[n] = true
				}
			}
			stmt := `insert into ` + qualified + ` (` + strings.Join(list, ", ") + `)
				overriding system value
				select ` + strings.Join(list, ", ") + `
				from jsonb_populate_recordset(null::` + qualified + `, $1::jsonb)`
			for start := 0; start < len(grp.rows); start += batchRows {
				end := min(start+batchRows, len(grp.rows))
				batch, err := json.Marshal(grp.rows[start:end])
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, stmt, string(batch)); err != nil {
					return fmt.Errorf("导入 %s：%w", t.Name, err)
				}
			}
		}
		// 回读核对，而不是相信计数：部分写入看起来也像成功。
		var after int64
		if err := tx.QueryRow(ctx, `select count(*) from `+qualified).Scan(&after); err != nil {
			return err
		}
		if after-before != int64(t.Rows) {
			return fmt.Errorf("导入 %s：写入了 %d 行，manifest 写的是 %d", t.Name, after-before, t.Rows)
		}
		// 带值写入的行不会推进序列，不调整的话应用下一次插入会与快照的行冲突。
		for n := range serial {
			_, err := tx.Exec(ctx, `select setval(pg_get_serial_sequence($1, $2), coalesce(max(`+pgx.Identifier{n}.Sanitize()+`), 0) + 1, false)
				from `+qualified, qualified, n)
			if err != nil {
				return fmt.Errorf("调整 %s.%s 的序列：%w", t.Name, n, err)
			}
		}
		fmt.Fprintf(r.Out, "  %-20s %6d 行\n", t.Name, t.Rows)
	}
	return nil
}

// rowGroup 是一个表里列出相同列的行。
type rowGroup struct {
	names []string
	rows  []json.RawMessage
}

// groupRows 按每行列出的列分组（按首次出现的顺序），并检查每一列都存在且可写。
func groupRows(table string, rows []json.RawMessage, cols map[string]column) ([]rowGroup, error) {
	var groups []rowGroup
	at := map[string]int{}
	for n, raw := range rows {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, fmt.Errorf("表 %s 第 %d 行：%w", table, n+1, err)
		}
		names := make([]string, 0, len(obj))
		for k := range obj {
			c, ok := cols[k]
			if !ok {
				return nil, fmt.Errorf("表 %s 没有列 %q——快照是不是从别的 schema 切出的？", table, k)
			}
			if c.Generated {
				return nil, fmt.Errorf("表 %s：%q 是生成列，不能导入", table, k)
			}
			names = append(names, k)
		}
		if len(names) == 0 {
			return nil, fmt.Errorf("表 %s 第 %d 行没有任何列", table, n+1)
		}
		sort.Strings(names)
		key := strings.Join(names, ",")
		i, ok := at[key]
		if !ok {
			i = len(groups)
			at[key] = i
			groups = append(groups, rowGroup{names: names})
		}
		groups[i].rows = append(groups[i].rows, raw)
	}
	return groups, nil
}

// provision 写入测试账号，密码都是 r.Password。
func (r Runner) provision(ctx context.Context, tx pgx.Tx, name string, accts *Accounts) error {
	for _, a := range accts.Accounts {
		hash, err := auth.HashPassword(r.Password)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `insert into public.app_user (tenant, email, display_name, role, password_hash)
			values ($1, $2, $3, $4, $5)`, name, a.Email, a.Name, string(a.Role), hash); err != nil {
			return fmt.Errorf("写入账号 %s：%w", a.Email, err)
		}
		fmt.Fprintf(r.Out, "  %-28s %s\n", a.Email, a.Role)
	}
	return nil
}

// tenantExists 报告租户的 schema 或登记是否存在（任一存在即视为存在）。
func tenantExists(ctx context.Context, q db.Querier, name string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `select exists (select 1 from information_schema.schemata where schema_name = $1)
		or exists (select 1 from public.tenant where name = $1)`, name).Scan(&exists)
	return exists, err
}
