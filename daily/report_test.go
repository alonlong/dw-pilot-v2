package daily

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// day 构造某天某时刻的时间（UTC），便于测试。
func day(month time.Month, d, hour int) time.Time {
	return time.Date(2026, month, d, hour, 0, 0, 0, time.UTC)
}

func TestNewReportTruncatesDate(t *testing.T) {
	r := NewReport("J1", day(10, 1, 15))
	if want := day(10, 1, 0); !r.BusinessDate.Equal(want) {
		t.Fatalf("BusinessDate = %v, want %v", r.BusinessDate, want)
	}
	if r.Submitted() {
		t.Fatal("new report should not be submitted")
	}
}

func TestSubmit(t *testing.T) {
	r := NewReport("J1", day(10, 1, 0))
	at := day(10, 1, 18)
	if err := r.Submit(at); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !r.SubmittedAt.Equal(at) {
		t.Fatalf("SubmittedAt = %v, want %v", r.SubmittedAt, at)
	}
	if err := r.Submit(day(10, 2, 9)); !errors.Is(err, ErrAlreadySubmitted) {
		t.Fatalf("second Submit err = %v, want ErrAlreadySubmitted", err)
	}
	if !r.SubmittedAt.Equal(at) {
		t.Fatal("second Submit must not change SubmittedAt")
	}
}

func TestUpdateUnlocked(t *testing.T) {
	tests := []struct {
		name    string
		crew    map[string]float64
		wantErr error
		want    float64
	}{
		{"set hours", map[string]float64{"张三": 8, "李四": 6.5}, nil, 14.5},
		{"negative hours rejected", map[string]float64{"张三": -1}, ErrNegativeHours, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReport("J1", day(10, 1, 0))
			err := r.Update(tt.crew)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Update err = %v, want %v", err, tt.wantErr)
			}
			if got := r.TotalHours(); got != tt.want {
				t.Fatalf("TotalHours = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendNote(t *testing.T) {
	tests := []struct {
		name   string
		locked bool
	}{
		{"unlocked", false},
		{"locked", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReport("J1", day(10, 1, 0))
			if tt.locked {
				r.Lock()
			}
			for _, n := range []string{"混凝土浇筑完成", "下午有雨"} {
				if err := r.AppendNote(n); err != nil {
					t.Fatalf("AppendNote(%q): %v", n, err)
				}
			}
			if len(r.Notes) != 2 || r.Notes[0] != "混凝土浇筑完成" || r.Notes[1] != "下午有雨" {
				t.Fatalf("Notes = %v", r.Notes)
			}
			if err := r.AppendNote(""); !errors.Is(err, ErrEmptyNote) {
				t.Fatalf("empty note err = %v, want ErrEmptyNote", err)
			}
		})
	}
}

func TestAppendNoteValidation(t *testing.T) {
	tests := []struct {
		name    string
		note    string
		wantErr error
		wantLen int
	}{
		{"blank spaces", "   ", ErrEmptyNote, 0},
		{"tabs and newlines", " \t\n", ErrEmptyNote, 0},
		{"500 chinese chars ok", strings.Repeat("字", 500), nil, 1},
		{"501 chinese chars too long", strings.Repeat("字", 501), ErrNoteTooLong, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReport("J1", day(10, 1, 0))
			err := r.AppendNote(tt.note)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AppendNote err = %v, want %v", err, tt.wantErr)
			}
			if len(r.Notes) != tt.wantLen {
				t.Fatalf("len(Notes) = %d, want %d", len(r.Notes), tt.wantLen)
			}
		})
	}
}

func TestLatestNotes(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want []string
	}{
		{"latest 2", 2, []string{"c", "b"}},
		{"n larger than total", 5, []string{"c", "b", "a"}},
		{"zero", 0, []string{}},
		{"negative", -1, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReport("J1", day(10, 1, 0))
			for _, n := range []string{"a", "b", "c"} {
				if err := r.AppendNote(n); err != nil {
					t.Fatal(err)
				}
			}
			if got := r.LatestNotes(tt.n); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LatestNotes(%d) = %v, want %v", tt.n, got, tt.want)
			}
			if !reflect.DeepEqual(r.Notes, []string{"a", "b", "c"}) {
				t.Fatalf("Notes order changed: %v", r.Notes)
			}
		})
	}
}

func TestLock(t *testing.T) {
	r := NewReport("J1", day(10, 1, 0))
	r.Lock()
	if !r.Locked {
		t.Fatal("Lock should set Locked")
	}
}

func TestLateness(t *testing.T) {
	tests := []struct {
		name      string
		submitted time.Time // 零值表示未提交
		wantLate  bool
	}{
		{"not submitted", time.Time{}, false},
		{"same day morning", day(10, 1, 8), false},
		{"same day late evening", day(10, 1, 23), false},
		{"very late", day(10, 10, 9), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReport("J1", day(10, 1, 0))
			if !tt.submitted.IsZero() {
				if err := r.Submit(tt.submitted); err != nil {
					t.Fatal(err)
				}
			}
			if got := r.IsLate(); got != tt.wantLate {
				t.Fatalf("IsLate = %v, want %v", got, tt.wantLate)
			}
			if !tt.wantLate && r.DaysLate() != 0 {
				t.Fatalf("DaysLate = %d, want 0 for on-time report", r.DaysLate())
			}
			if tt.wantLate && r.DaysLate() <= 0 {
				t.Fatalf("DaysLate = %d, want > 0 for late report", r.DaysLate())
			}
		})
	}
}
