package tenant

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSlot(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"dlpv1", true},
		{"dlpv97", true},
		{"dlpv999", true},
		{"dlpv", false},
		{"dlpv1000", false},
		{"dlpvault", false},
		{"dlpv_1", false},
		{"dlpv1x", false},
		{"xdlpv1", false},
		{"DLPV1", false},
		{"pmpv1", false},
		{"dev", false},
		{"public", false},
		{"", false},
		{"dlpv1; drop schema public", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSlot(tt.name); got != tt.want {
				t.Errorf("IsSlot(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestRunnerRefusesNonSlot 检查非槽位名字在连接数据库之前就被拒绝（DSN 是连不上的地址）。
func TestRunnerRefusesNonSlot(t *testing.T) {
	r := Runner{DSN: "postgres://nobody@127.0.0.1:1/none", Out: io.Discard, Password: "x"}
	snap, err := LoadSnapshot(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, name := range []string{"public", "dev", "dlpv", "dlpvault", "abby", "pg_catalog", ""} {
		accts := DefaultAccounts(name)
		ops := map[string]func() error{
			"seed":  func() error { return r.Seed(ctx, name, snap, accts) },
			"reset": func() error { return r.Reset(ctx, name, snap, accts) },
			"drop":  func() error { return r.Drop(ctx, name) },
		}
		for op, fn := range ops {
			t.Run(op+" "+name, func(t *testing.T) {
				err := fn()
				if err == nil || !strings.Contains(err.Error(), "不是预览槽位") {
					t.Errorf("err = %v, want 不是预览槽位", err)
				}
			})
		}
	}
}

func TestRunnerPrecheck(t *testing.T) {
	snap, err := LoadSnapshot(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	oldSnap := *snap
	oldSnap.Manifest.TenantMigration = 99
	ctx := context.Background()
	tests := []struct {
		name     string
		password string
		snap     *Snapshot
		accts    *Accounts
		want     string
	}{
		{"没有密码", "", snap, DefaultAccounts("dlpv5"), "TENANT_TEST_PASSWORD"},
		{"快照的迁移版本不对", "x", &oldSnap, DefaultAccounts("dlpv5"), "租户迁移 99"},
		{"账号属于别的租户", "x", snap, DefaultAccounts("dlpv6"), "不是 \"dlpv5\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Runner{DSN: "postgres://nobody@127.0.0.1:1/none", Out: io.Discard, Password: tt.password}
			err := r.Reset(ctx, "dlpv5", tt.snap, tt.accts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want 含 %q", err, tt.want)
			}
		})
	}
}

// copyGolden 把 Golden 快照复制到临时目录，返回目录路径。
func copyGolden(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(goldenDir)); err != nil {
		t.Fatal(err)
	}
	return dir
}

func editFile(t *testing.T, path string, fn func(string) string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(fn(string(raw))), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSnapshotRejects(t *testing.T) {
	jobSHA := func(dir string) string {
		raw, err := os.ReadFile(filepath.Join(dir, "tables/job.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		return FileSHA256(raw)
	}
	tests := []struct {
		name string
		edit func(t *testing.T, dir string)
		want string
	}{
		{"没有 manifest", func(t *testing.T, dir string) {
			os.Remove(filepath.Join(dir, "manifest.json"))
		}, "manifest"},
		{"未知字段", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"format"`, `"formt": 1, "format"`, 1)
			})
		}, "unknown field"},
		{"格式版本不对", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"format": 1`, `"format": 2`, 1)
			})
		}, "格式是 2"},
		{"没有 tenant_migration", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"tenant_migration": 1`, `"tenant_migration": 0`, 1)
			})
		}, "tenant_migration"},
		{"数据文件被改动（sha256 不符）", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "tables/report.jsonl"), func(s string) string {
				return strings.Replace(s, `"locked": true`, `"locked": false`, 1)
			})
		}, "sha256"},
		{"行数不符", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"rows": 3,`, `"rows": 4,`, 1)
			})
		}, "有 3 行，manifest 写的是 4"},
		{"文件不存在", func(t *testing.T, dir string) {
			os.Remove(filepath.Join(dir, "tables/crew_hours.jsonl"))
		}, "crew_hours"},
		{"路径跑出快照目录", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"tables/job.jsonl"`, `"../job.jsonl"`, 1)
			})
		}, "快照目录之外"},
		{"不能来自快照的表", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"name": "job"`, `"name": "app_user"`, 1)
			})
		}, "不能来自快照"},
		{"表名不合法", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"name": "job"`, `"name": "job; drop"`, 1)
			})
		}, "不是普通的表名"},
		{"表列了两次", func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				return strings.Replace(s, `"name": "report",`, `"name": "job",`, 1)
			})
		}, "列了两次"},
		{"有一行不是 JSON 对象", func(t *testing.T, dir string) {
			p := filepath.Join(dir, "tables/job.jsonl")
			editFile(t, p, func(s string) string { return strings.Replace(s, "\n", "\n[1]\n", 1) })
			sha := jobSHA(dir)
			editFile(t, filepath.Join(dir, "manifest.json"), func(s string) string {
				i := strings.Index(s, `"tables/job.jsonl"`)
				j := strings.Index(s[i:], `"sha256": "`) + i + len(`"sha256": "`)
				return s[:j] + sha + s[j+64:]
			})
		}, "不是 JSON 对象"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyGolden(t)
			tt.edit(t, dir)
			_, err := LoadSnapshot(dir)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want 含 %q", err, tt.want)
			}
		})
	}
}

func TestLoadAccounts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		tenant  string
		want    string // 为空表示成功
		count   int
	}{
		{"正常", `{"tenant":"dlpv1","accounts":[{"email":"A@x.test","name":"甲","role":"admin"},{"email":"b@x.test","name":"乙","role":"finance"}]}`, "dlpv1", "", 2},
		{"租户不符", `{"tenant":"dlpv2","accounts":[{"email":"a@x.test","name":"甲","role":"admin"}]}`, "dlpv1", "不是 \"dlpv1\"", 0},
		{"角色不对", `{"tenant":"dlpv1","accounts":[{"email":"a@x.test","name":"甲","role":"pm"}]}`, "dlpv1", "角色", 0},
		{"邮箱不对", `{"tenant":"dlpv1","accounts":[{"email":"ax.test","name":"甲","role":"admin"}]}`, "dlpv1", "邮箱", 0},
		{"没有名字", `{"tenant":"dlpv1","accounts":[{"email":"a@x.test","name":" ","role":"admin"}]}`, "dlpv1", "没有名字", 0},
		{"邮箱重复（大小写不同）", `{"tenant":"dlpv1","accounts":[{"email":"a@x.test","name":"甲","role":"admin"},{"email":"A@X.test","name":"乙","role":"field"}]}`, "dlpv1", "两次", 0},
		{"没有账号", `{"tenant":"dlpv1","accounts":[]}`, "dlpv1", "没有账号", 0},
		{"未知字段", `{"tenant":"dlpv1","accounts":[{"email":"a@x.test","name":"甲","role":"admin","password":"x"}]}`, "dlpv1", "unknown field", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "accounts.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			a, err := LoadAccounts(path, tt.tenant)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Errorf("err = %v, want 含 %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Accounts) != tt.count {
				t.Errorf("len = %d, want %d", len(a.Accounts), tt.count)
			}
			if a.Accounts[0].Email != "a@x.test" {
				t.Errorf("邮箱没有转成小写：%q", a.Accounts[0].Email)
			}
		})
	}
}

func TestDefaultAccounts(t *testing.T) {
	a := DefaultAccounts("dlpv1")
	want := []string{"admin@dlpv1.test", "manager@dlpv1.test", "field@dlpv1.test", "finance@dlpv1.test"}
	if a.Tenant != "dlpv1" || len(a.Accounts) != len(want) {
		t.Fatalf("accounts = %+v", a)
	}
	for i, acct := range a.Accounts {
		if acct.Email != want[i] || acct.Name == "" || !acct.Role.Valid() {
			t.Errorf("accounts[%d] = %+v", i, acct)
		}
	}
	if err := a.validate(); err != nil {
		t.Error(err)
	}
}
