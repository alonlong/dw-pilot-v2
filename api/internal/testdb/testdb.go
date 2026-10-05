// Package testdb 为需要数据库的测试建立租户。只在测试里使用。
//
// 数据库取环境变量 TEST_DATABASE_URL，没有时跳过测试。每个测试包用不同的槽位名
// （go test 会并行跑多个包），测试结束时删除。
package testdb

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/dailylog/api/internal/db"
	"example.com/dailylog/api/internal/tenant"
)

// Password 是测试账号的密码。
const Password = "test-password"

// URL 返回 TEST_DATABASE_URL；没有时跳过测试。
func URL(t testing.TB) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL 未设置，跳过需要数据库的测试")
	}
	return url
}

// GoldenDir 返回 Golden 快照目录的绝对路径。
func GoldenDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "golden")
}

// Pool 返回连到测试库的连接池（已跑 public 迁移），测试结束时关闭。
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := URL(t)
	ctx := context.Background()
	if err := db.MigratePublic(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Seed 按 Golden 快照重建槽位 name，带四个默认账号（密码 Password），测试结束时删除。
func Seed(t testing.TB, name string) {
	t.Helper()
	url := URL(t)
	snap, err := tenant.LoadSnapshot(GoldenDir())
	if err != nil {
		t.Fatal(err)
	}
	r := tenant.Runner{DSN: url, Out: io.Discard, Password: Password}
	if err := r.Reset(context.Background(), name, snap, tenant.DefaultAccounts(name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Drop(context.Background(), name) })
}
