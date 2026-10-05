package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPassword(t *testing.T) {
	hash, err := HashPassword("正确的密码")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("哈希格式不对：%s", hash)
	}
	other, err := HashPassword("正确的密码")
	if err != nil {
		t.Fatal(err)
	}
	if other == hash {
		t.Error("同一密码两次哈希相同：盐没有随机")
	}

	parts := strings.Split(hash, "$")
	tests := []struct {
		name     string
		hash     string
		password string
		want     bool
		wantErr  error
	}{
		{"正确密码", hash, "正确的密码", true, nil},
		{"错误密码", hash, "错误的密码", false, nil},
		{"空密码", hash, "", false, nil},
		{"大小写不同", hash, "正确的密码 ", false, nil},
		{"不是 argon2id", strings.Replace(hash, "argon2id", "argon2i", 1), "正确的密码", false, ErrBadHash},
		{"段数不对", "$argon2id$v=19$abc", "正确的密码", false, ErrBadHash},
		{"版本不对", strings.Replace(hash, "v=19", "v=16", 1), "正确的密码", false, ErrBadHash},
		{"参数为零", strings.Replace(hash, "t=2", "t=0", 1), "正确的密码", false, ErrBadHash},
		{"盐不是 base64", strings.Join([]string{"", parts[1], parts[2], parts[3], "!!!", parts[5]}, "$"), "正确的密码", false, ErrBadHash},
		{"空字符串", "", "正确的密码", false, ErrBadHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.hash, tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("VerifyPassword = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Error("空密码应当返回错误")
	}
}
