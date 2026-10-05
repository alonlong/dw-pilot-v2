package auth

import "testing"

func TestRoleCan(t *testing.T) {
	// 与 docs/api.md 的权限表逐格对应。
	tests := []struct {
		role Role
		want map[Action]bool
	}{
		{Admin, map[Action]bool{View: true, EditReport: true, Lock: true, AddNote: true, ViewSummary: true}},
		{Manager, map[Action]bool{View: true, EditReport: false, Lock: true, AddNote: true, ViewSummary: true}},
		{Field, map[Action]bool{View: true, EditReport: true, Lock: false, AddNote: true, ViewSummary: false}},
		{Finance, map[Action]bool{View: true, EditReport: false, Lock: false, AddNote: false, ViewSummary: true}},
		{Role("owner"), map[Action]bool{View: false, EditReport: false, Lock: false, AddNote: false, ViewSummary: false}},
	}
	for _, tt := range tests {
		for a, want := range tt.want {
			t.Run(string(tt.role)+"/"+string(a), func(t *testing.T) {
				if got := tt.role.Can(a); got != want {
					t.Errorf("%s.Can(%s) = %v, want %v", tt.role, a, got, want)
				}
			})
		}
	}
}

func TestRoleValid(t *testing.T) {
	tests := []struct {
		role Role
		want bool
	}{
		{Admin, true}, {Manager, true}, {Field, true}, {Finance, true},
		{"", false}, {"Admin", false}, {"owner", false},
	}
	for _, tt := range tests {
		if got := tt.role.Valid(); got != tt.want {
			t.Errorf("Role(%q).Valid() = %v, want %v", tt.role, got, tt.want)
		}
	}
}
