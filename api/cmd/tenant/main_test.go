package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

const golden = "../../testdata/golden"

func TestRun(t *testing.T) {
	// DSN 指向连不上的地址：被拒绝的调用不应走到连接数据库那一步。
	const dsn = "postgres://nobody@127.0.0.1:1/none"
	tests := []struct {
		name     string
		sub      string
		tenant   string
		snapshot string
		password string
		want     string // 为空表示成功
	}{
		{"check 通过", "check", "", golden, "", ""},
		{"check 缺 -snapshot", "check", "", "", "", "需要 -snapshot"},
		{"check 快照目录不存在", "check", "", "/nonexistent", "", "manifest"},
		{"seed 非槽位", "seed", "dev", golden, "x", "不是预览槽位"},
		{"reset 非槽位", "reset", "public", golden, "x", "不是预览槽位"},
		{"drop 非槽位", "drop", "abby", "", "x", "不是预览槽位"},
		{"drop 空名字", "drop", "", "", "x", "不是预览槽位"},
		{"reset 没有密码", "reset", "dlpv1", golden, "", "TENANT_TEST_PASSWORD"},
		{"seed 缺 -snapshot", "seed", "dlpv1", "", "x", "需要 -snapshot"},
		{"未知子命令", "migrate", "dlpv1", "", "x", "未知的子命令"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(context.Background(), &out, tt.sub, tt.tenant, tt.snapshot, "", dsn, tt.password)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), "report                   30 行") {
					t.Errorf("输出 = %s", out.String())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want 含 %q", err, tt.want)
			}
		})
	}
}
