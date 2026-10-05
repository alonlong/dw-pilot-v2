package tenant

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/dailylog/api/internal/db"
)

// liveRunner 返回连到 TEST_DATABASE_URL 的 Runner；没有时跳过。
func liveRunner(t *testing.T) (Runner, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL 未设置，跳过需要数据库的测试")
	}
	ctx := context.Background()
	if err := db.MigratePublic(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return Runner{DSN: url, Out: io.Discard, Password: "live-test-password"}, pool
}

// fingerprint 汇总租户的全部数据：快照的每张表、序列位置、账号（不含 id 和密码哈希）。
func fingerprint(t *testing.T, pool *pgxpool.Pool, name string, snap *Snapshot) string {
	t.Helper()
	ctx := context.Background()
	var b strings.Builder
	for _, tb := range snap.Manifest.Tables {
		q := pgx.Identifier{name, tb.Name}.Sanitize()
		var s string
		err := pool.QueryRow(ctx, `select coalesce(string_agg(x, E'\n' order by x), '') from (select row_to_json(t)::text as x from `+q+` t) s`).Scan(&s)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "== %s\n%s\n", tb.Name, s)
	}
	for _, seq := range []string{"report", "report_note"} {
		var next int64
		err := pool.QueryRow(ctx, `select nextval(pg_get_serial_sequence($1, 'id'))`, pgx.Identifier{name, seq}.Sanitize()).Scan(&next)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "== %s next id %d\n", seq, next)
	}
	var users string
	err := pool.QueryRow(ctx, `select coalesce(string_agg(email || ' ' || role || ' ' || display_name || ' ' || disabled::text, E'\n' order by email), '')
		from public.app_user where tenant = $1`, name).Scan(&users)
	if err != nil {
		t.Fatal(err)
	}
	var tenantName string
	if err := pool.QueryRow(ctx, `select display_name from public.tenant where name = $1`, name).Scan(&tenantName); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(&b, "== users\n%s\n== tenant %s\n", users, tenantName)
	return b.String()
}

func gone(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	exists, err := tenantExists(context.Background(), pool, name)
	if err != nil {
		t.Fatal(err)
	}
	var users int
	if err := pool.QueryRow(context.Background(), `select count(*) from public.app_user where tenant = $1`, name).Scan(&users); err != nil {
		t.Fatal(err)
	}
	return !exists && users == 0
}

// TestSeedResetLive：seed 一个槽位，再 seed 被拒绝，reset 两次结果都与 seed 相同，最后 drop。
func TestSeedResetLive(t *testing.T) {
	r, pool := liveRunner(t)
	const slot = "dlpv961"
	ctx := context.Background()
	snap, err := LoadSnapshot(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	accts := DefaultAccounts(slot)
	if err := r.Drop(ctx, slot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Drop(context.Background(), slot) })

	if err := r.Seed(ctx, slot, snap, accts); err != nil {
		t.Fatal(err)
	}
	seeded := fingerprint(t, pool, slot, snap)
	if !strings.Contains(seeded, "admin@dlpv961.test admin") || !strings.Contains(seeded, "== report next id 31") {
		// 快照的日报 id 是 1～30，序列应当从 31 开始。
		t.Errorf("seed 的结果不对：\n%s", seeded)
	}
	if err := r.Seed(ctx, slot, snap, accts); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Errorf("第二次 seed 应当被拒绝，err = %v", err)
	}
	v, err := db.TenantVersion(ctx, pool, slot)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := db.TenantTarget(); v != want {
		t.Errorf("租户迁移版本 = %d, want %d", v, want)
	}

	for i := 1; i <= 2; i++ {
		// 改点数据，reset 应当恢复。
		if _, err := pool.Exec(ctx, `update `+pgx.Identifier{slot, "report"}.Sanitize()+` set locked = true`); err != nil {
			t.Fatal(err)
		}
		if err := r.Reset(ctx, slot, snap, accts); err != nil {
			t.Fatal(err)
		}
		if got := fingerprint(t, pool, slot, snap); got != seeded {
			t.Fatalf("第 %d 次 reset 的结果与 seed 不同：\nseed:\n%s\nreset:\n%s", i, seeded, got)
		}
	}

	if err := r.Drop(ctx, slot); err != nil {
		t.Fatal(err)
	}
	if !gone(t, pool, slot) {
		t.Error("drop 之后租户仍然存在")
	}
	if err := r.Drop(ctx, slot); err != nil {
		t.Errorf("删除不存在的槽位应当成功，err = %v", err)
	}
}

// TestSeedFailureCleansUp：导入中途失败时，建了一半的租户被删掉。
func TestSeedFailureCleansUp(t *testing.T) {
	r, pool := liveRunner(t)
	const slot = "dlpv962"
	ctx := context.Background()
	t.Cleanup(func() { _ = r.Drop(context.Background(), slot) })

	tests := []struct {
		name string
		edit func(s string) string
		file string
		want string
	}{
		{"未知列", func(s string) string { return strings.Replace(s, `"address"`, `"addr"`, 1) }, "job", "没有列"},
		{"违反约束（负工时）", func(s string) string { return strings.Replace(s, `"hours": 8`, `"hours": -8`, 1) }, "crew_hours", "导入 crew_hours"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyGolden(t)
			p := filepath.Join(dir, "tables", tt.file+".jsonl")
			editFile(t, p, tt.edit)
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				i := strings.Index(s, `"tables/`+tt.file+`.jsonl"`)
				j := strings.Index(s[i:], `"sha256": "`) + i + len(`"sha256": "`)
				return s[:j] + FileSHA256(raw) + s[j+64:]
			})
			snap, err := LoadSnapshot(dir)
			if err != nil {
				t.Fatal(err)
			}
			err = r.Reset(ctx, slot, snap, DefaultAccounts(slot))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want 含 %q", err, tt.want)
			}
			if !gone(t, pool, slot) {
				t.Error("失败后租户没有被清理")
			}
		})
	}
}
