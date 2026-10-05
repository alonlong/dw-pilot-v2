package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/dailylog/api/internal/auth"
	"example.com/dailylog/api/internal/testdb"
)

const slot = "dlpv971"

func TestLogin(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slot)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `update public.app_user set disabled = true where tenant = $1 and role = 'finance'`, slot); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		email    string
		password string
		wantRole auth.Role
		wantErr  error
	}{
		{"管理员", "admin@" + slot + ".test", testdb.Password, auth.Admin, nil},
		{"邮箱大小写与空格不敏感", "  Manager@" + slot + ".TEST ", testdb.Password, auth.Manager, nil},
		{"密码错误", "field@" + slot + ".test", "wrong", "", auth.ErrBadCredentials},
		{"邮箱不存在", "nobody@" + slot + ".test", testdb.Password, "", auth.ErrBadCredentials},
		{"空邮箱", "", testdb.Password, "", auth.ErrBadCredentials},
		{"空密码", "admin@" + slot + ".test", "", "", auth.ErrBadCredentials},
		{"已停用", "finance@" + slot + ".test", testdb.Password, "", auth.ErrBadCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := auth.Login(ctx, pool, tt.email, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if u.Role != tt.wantRole || u.Tenant != slot || u.TenantName == "" {
				t.Errorf("user = %+v", u)
			}
		})
	}
}

func TestSession(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, "dlpv972")
	ctx := context.Background()
	u, err := auth.Login(ctx, pool, "field@dlpv972.test", testdb.Password)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	token, expires, err := auth.NewSession(ctx, pool, u.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if !expires.Equal(now.Add(12 * time.Hour)) {
		t.Errorf("expires = %v, want 12 小时后", expires)
	}

	tests := []struct {
		name    string
		token   string
		at      time.Time
		wantErr error
	}{
		{"有效", token, now.Add(time.Hour), nil},
		{"到期前一秒", token, now.Add(12*time.Hour - time.Second), nil},
		{"恰好到期", token, now.Add(12 * time.Hour), auth.ErrNoSession},
		{"过期", token, now.Add(13 * time.Hour), auth.ErrNoSession},
		{"空令牌", "", now, auth.ErrNoSession},
		{"伪造令牌", "not-a-real-token", now, auth.ErrNoSession},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auth.LookupSession(ctx, pool, tt.token, tt.at)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got.ID != u.ID {
				t.Errorf("user id = %d, want %d", got.ID, u.ID)
			}
		})
	}

	t.Run("停用用户的会话失效", func(t *testing.T) {
		tok, _, err := auth.NewSession(ctx, pool, u.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `update public.app_user set disabled = true where id = $1`, u.ID); err != nil {
			t.Fatal(err)
		}
		defer pool.Exec(ctx, `update public.app_user set disabled = false where id = $1`, u.ID)
		if _, err := auth.LookupSession(ctx, pool, tok, now); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("err = %v, want ErrNoSession", err)
		}
	})

	t.Run("登出后失效", func(t *testing.T) {
		if err := auth.DeleteSession(ctx, pool, token); err != nil {
			t.Fatal(err)
		}
		if _, err := auth.LookupSession(ctx, pool, token, now); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("err = %v, want ErrNoSession", err)
		}
	})

	t.Run("新建会话时清理过期会话", func(t *testing.T) {
		if _, _, err := auth.NewSession(ctx, pool, u.ID, now); err != nil {
			t.Fatal(err)
		}
		if _, _, err := auth.NewSession(ctx, pool, u.ID, now.Add(24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from public.session where user_id = $1 and expires_at <= $2`, u.ID, now.Add(24*time.Hour)).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("还有 %d 个过期会话", n)
		}
	})
}
