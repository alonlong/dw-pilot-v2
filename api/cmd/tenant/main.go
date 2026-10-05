// Command tenant 建立、重建、删除预览槽位租户，契约与 abby-pm 的 cmd/tenant 一致：
//
//	tenant check -snapshot api/testdata/golden
//	tenant seed  -tenant dlpv1 -snapshot <dir> [-accounts <file>]
//	tenant reset -tenant dlpv1 -snapshot <dir> [-accounts <file>]
//	tenant drop  -tenant dlpv1
//
//	check   读取快照并按 manifest 校验；不连接数据库
//	seed    建立不存在的槽位
//	reset   删除槽位（如存在）再建立；每次结果相同
//	drop    删除槽位及其用户、会话
//
// 只处理槽位：seed、reset、drop 只接受 dlpv 加数字的名字，其他名字一律拒绝，没有开关。
// 本地开发租户 dev 用 cmd/devseed 建立。
//
// 账号文件格式见 internal/tenant.Accounts；不给 -accounts 时按四种角色生成默认账号
// （admin@<tenant>.test 等）。测试账号的密码取环境变量 TENANT_TEST_PASSWORD，没有时拒绝执行。
// 数据库取 DATABASE_URL。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/dailylog/api/internal/tenant"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	sub := os.Args[1]
	fs := flag.NewFlagSet(sub, flag.ExitOnError)
	name := fs.String("tenant", "", "槽位："+tenant.SlotPrefix+" 加数字")
	snapshot := fs.String("snapshot", os.Getenv("TENANT_SNAPSHOT"), "快照目录（含 manifest.json）")
	accounts := fs.String("accounts", "", "账号文件；不给时按四种角色生成默认账号")
	_ = fs.Parse(os.Args[2:])

	err := run(context.Background(), os.Stdout, sub, *name, *snapshot, *accounts,
		os.Getenv("DATABASE_URL"), os.Getenv("TENANT_TEST_PASSWORD"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "用法：tenant check|seed|reset|drop [-tenant dlpvN] [-snapshot dir] [-accounts file]")
}

func run(ctx context.Context, out io.Writer, sub, name, snapshot, accounts, dsn, password string) error {
	switch sub {
	case "check":
		if snapshot == "" {
			return errors.New("需要 -snapshot")
		}
		snap, err := tenant.LoadSnapshot(snapshot)
		if err != nil {
			return err
		}
		m := snap.Manifest
		fmt.Fprintf(out, "%s：格式 %d，租户迁移 %d，%d 张表\n", m.Name, m.Format, m.TenantMigration, len(m.Tables))
		for _, t := range m.Tables {
			fmt.Fprintf(out, "  %-20s %6d 行\n", t.Name, t.Rows)
		}
		if name != "" && accounts != "" {
			a, err := tenant.LoadAccounts(accounts, name)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s 有 %d 个账号\n", name, len(a.Accounts))
		}
		return nil

	case "seed", "reset", "drop":
		// 在读取任何东西、连接数据库之前拒绝，名字不对时无论其他参数如何都同样失败。
		if !tenant.IsSlot(name) {
			return fmt.Errorf("-tenant %q 不是预览槽位（%s 加数字）", name, tenant.SlotPrefix)
		}
		if dsn == "" {
			return errors.New("DATABASE_URL 未设置")
		}
		r := tenant.Runner{DSN: dsn, Out: out, Password: password}
		if sub == "drop" {
			return r.Drop(ctx, name)
		}
		if password == "" {
			return tenant.ErrNoPassword
		}
		if snapshot == "" {
			return fmt.Errorf("%s 需要 -snapshot（或 TENANT_SNAPSHOT）", sub)
		}
		snap, err := tenant.LoadSnapshot(snapshot)
		if err != nil {
			return err
		}
		accts := tenant.DefaultAccounts(name)
		if accounts != "" {
			if accts, err = tenant.LoadAccounts(accounts, name); err != nil {
				return err
			}
		}
		if sub == "seed" {
			return r.Seed(ctx, name, snap, accts)
		}
		return r.Reset(ctx, name, snap, accts)

	default:
		usage(out)
		return fmt.Errorf("未知的子命令 %q", sub)
	}
}
