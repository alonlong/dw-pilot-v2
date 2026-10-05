// Command devseed 建立本地开发用的租户 dev，仅供本地开发：
//
//	devseed [-reset] [-snapshot api/testdata/golden]
//
// 默认在 dev 不存在时建立，已存在则拒绝；-reset 删除并重建 dev。它只会处理名为 dev 的
// 租户（代码里的常量），不接受租户名参数，所以不能用来删除别的 schema；槽位租户用 cmd/tenant。
// 账号按四种角色生成（admin@dev.test 等），密码取 TENANT_TEST_PASSWORD，数据库取 DATABASE_URL。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"example.com/dailylog/api/internal/tenant"
)

func main() {
	reset := flag.Bool("reset", false, "删除并重建 dev 租户")
	snapshot := flag.String("snapshot", "api/testdata/golden", "快照目录（含 manifest.json）")
	flag.Parse()
	if err := run(*reset, *snapshot); err != nil {
		fmt.Fprintln(os.Stderr, "devseed:", err)
		os.Exit(1)
	}
}

func run(reset bool, snapshot string) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL 未设置")
	}
	password := os.Getenv("TENANT_TEST_PASSWORD")
	if password == "" {
		return tenant.ErrNoPassword
	}
	snap, err := tenant.LoadSnapshot(snapshot)
	if err != nil {
		return err
	}
	r := tenant.Runner{DSN: dsn, Out: os.Stdout, Password: password}
	accts := tenant.DefaultAccounts(tenant.DevTenant)
	ctx := context.Background()
	if reset {
		return r.ResetDev(ctx, snap, accts)
	}
	return r.SeedDev(ctx, snap, accts)
}
