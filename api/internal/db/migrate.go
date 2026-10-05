package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"example.com/dailylog/api/migrations"
)

// MigratePublic 把 public schema 迁移到最新版本。多个副本同时启动时由 Goose 的
// 会话锁串行执行。
func MigratePublic(ctx context.Context, url string) error {
	return migrate(ctx, url, "public", "public")
}

// MigrateTenant 把租户 schema 迁移到最新版本；schema 必须已存在。
// Goose 的版本表放在租户 schema 里，每个租户各自记录版本。
func MigrateTenant(ctx context.Context, url, tenant string) error {
	if !ValidTenantName(tenant) {
		return fmt.Errorf("%w: %q", ErrBadTenant, tenant)
	}
	return migrate(ctx, url, tenant, "tenant")
}

func migrate(ctx context.Context, url, schema, dir string) error {
	sqlDB, err := openSQL(url, schema)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	p, err := newProvider(sqlDB, dir)
	if err != nil {
		return err
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("迁移 %s：%w", schema, err)
	}
	return nil
}

func newProvider(sqlDB *sql.DB, dir string) (*goose.Provider, error) {
	sub, err := fs.Sub(migrations.FS, dir)
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, sqlDB, sub,
		goose.WithSessionLocker(sessionLocker{}),
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(slog.New(slog.DiscardHandler)),
	)
}

// migrationLockID 是迁移用的 advisory lock 键（任意固定值）。
const migrationLockID int64 = 0x646c_6d69_6772 // "dlmigr"

// sessionLocker 用阻塞的 pg_advisory_lock 串行执行迁移。Goose 自带的会话锁是轮询
// pg_try_advisory_lock，最短间隔 1 秒，并发启动或并行测试时会白等。
type sessionLocker struct{}

func (sessionLocker) SessionLock(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `select pg_advisory_lock($1)`, migrationLockID)
	return err
}

func (sessionLocker) SessionUnlock(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `select pg_advisory_unlock($1)`, migrationLockID)
	return err
}

// openSQL 打开一个 database/sql 连接池，连接的 search_path 固定为 schema。
// schema 已校验过（public 或合法租户名）。
func openSQL(url, schema string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("解析 DATABASE_URL：%w", err)
	}
	cfg.RuntimeParams["search_path"] = schema
	return stdlib.OpenDB(*cfg), nil
}

// TenantTarget 返回代码里最新的租户迁移版本。
func TenantTarget() (int64, error) {
	return latestVersion("tenant")
}

// PublicTarget 返回代码里最新的 public 迁移版本。
func PublicTarget() (int64, error) {
	return latestVersion("public")
}

func latestVersion(dir string) (int64, error) {
	sub, err := fs.Sub(migrations.FS, dir)
	if err != nil {
		return 0, err
	}
	files, err := fs.Glob(sub, "*.sql")
	if err != nil {
		return 0, err
	}
	var max int64
	for _, f := range files {
		v, err := goose.NumericComponent(f)
		if err != nil {
			return 0, fmt.Errorf("迁移文件 %s/%s：%w", dir, f, err)
		}
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return 0, fmt.Errorf("%s 下没有迁移文件", dir)
	}
	return max, nil
}

// ErrNoTenantSchema 表示租户已登记但 schema 或版本表不存在。
var ErrNoTenantSchema = errors.New("db: tenant schema is missing")

// TenantVersion 返回租户 schema 已应用的最新迁移版本。
func TenantVersion(ctx context.Context, q Querier, tenant string) (int64, error) {
	ident, err := QuoteTenant(tenant)
	if err != nil {
		return 0, err
	}
	var exists bool
	err = q.QueryRow(ctx, `select exists (select 1 from pg_tables where schemaname = $1 and tablename = 'goose_db_version')`, tenant).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, fmt.Errorf("%w: %s", ErrNoTenantSchema, tenant)
	}
	var v int64
	err = q.QueryRow(ctx, `select coalesce(max(version_id), 0) from `+ident+`.goose_db_version where is_applied`).Scan(&v)
	return v, err
}
