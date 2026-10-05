// Package db 提供连接池、按租户切换 search_path 的事务，以及迁移入口。
//
// 租户隔离用 schema-per-tenant：每个租户一个与租户名同名的 schema。请求在事务内
// SET LOCAL search_path，事务结束设置即失效，不会带到连接池里下一个借用者。
package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// reTenant 与 public.tenant.name 的约束一致。
var reTenant = regexp.MustCompile(`^[a-z][a-z0-9]{1,30}$`)

// ValidTenantName 报告 name 是否是合法的租户名（也是 schema 名）。
func ValidTenantName(name string) bool { return reTenant.MatchString(name) }

// ErrBadTenant 表示租户名不合法；不合法的名字不会被拼进任何 SQL。
var ErrBadTenant = errors.New("db: invalid tenant name")

// Querier 是连接池、连接和事务共有的查询方法。
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Open 建立连接池并确认数据库可连通。
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("连接 Postgres：%w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("连接 Postgres：%w", err)
	}
	return pool, nil
}

// QuoteTenant 校验租户名并返回引用后的标识符，供 DDL 和 search_path 使用。
func QuoteTenant(tenant string) (string, error) {
	if !ValidTenantName(tenant) {
		return "", fmt.Errorf("%w: %q", ErrBadTenant, tenant)
	}
	return pgx.Identifier{tenant}.Sanitize(), nil
}

// InTenant 在一个事务里执行 fn，事务内 search_path 只含该租户的 schema
// （不含 public：漏写 schema 的 public 表会直接报错，而不是读错地方）。
// fn 返回错误时回滚，否则提交。
func InTenant(ctx context.Context, pool *pgxpool.Pool, tenant string, fn func(pgx.Tx) error) error {
	ident, err := QuoteTenant(tenant)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "set local search_path to "+ident); err != nil {
			return fmt.Errorf("切换到租户 %s：%w", tenant, err)
		}
		return fn(tx)
	})
}

// IsUniqueViolation 报告 err 是否是唯一约束冲突。
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
