package db

import (
	"errors"
	"testing"
)

func TestQuoteTenant(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"dlpv1", `"dlpv1"`, false},
		{"dev", `"dev"`, false},
		{"a", "", true},
		{"1abc", "", true},
		{"Dlpv1", "", true},
		{"dl_pv", "", true},
		{`x"; drop schema public; --`, "", true},
		{"abcdefghijklmnopqrstuvwxyz123456", "", true}, // 32 个字符，超过上限 31
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QuoteTenant(tt.name)
			if tt.wantErr {
				if !errors.Is(err, ErrBadTenant) {
					t.Errorf("err = %v, want ErrBadTenant", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("QuoteTenant(%q) = %q, %v", tt.name, got, err)
			}
		})
	}
}

func TestTargets(t *testing.T) {
	for name, fn := range map[string]func() (int64, error){"tenant": TenantTarget, "public": PublicTarget} {
		v, err := fn()
		if err != nil || v < 1 {
			t.Errorf("%s target = %d, %v", name, v, err)
		}
	}
}
