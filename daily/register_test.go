package daily

import (
	"testing"
)

func TestRegister(t *testing.T) {
	draft := NewReport("J2", day(10, 3, 0))

	submitted := NewReport("J1", day(10, 2, 0))
	if err := submitted.Submit(day(10, 2, 17)); err != nil {
		t.Fatal(err)
	}

	locked := NewReport("J3", day(10, 1, 0))
	if err := locked.Submit(day(10, 1, 18)); err != nil {
		t.Fatal(err)
	}
	locked.Lock()

	sameDay := NewReport("J0", day(10, 2, 0))

	rows := Register([]Report{*draft, *submitted, *locked, *sameDay})

	want := []struct {
		jobID, date, status string
	}{
		{"J3", "2026-10-01", StatusLocked},
		{"J0", "2026-10-02", StatusDraft},
		{"J1", "2026-10-02", StatusSubmitted},
		{"J2", "2026-10-03", StatusDraft},
	}
	if len(rows) != len(want) {
		t.Fatalf("len(rows) = %d, want %d", len(rows), len(want))
	}
	for i, w := range want {
		got := rows[i]
		if got.JobID != w.jobID || got.BusinessDate != w.date || got.Status != w.status {
			t.Errorf("rows[%d] = %+v, want {JobID:%s BusinessDate:%s Status:%s}", i, got, w.jobID, w.date, w.status)
		}
	}
}

func TestRegisterDaysLate(t *testing.T) {
	late := NewReport("J-1", day(10, 1, 0))
	if err := late.Submit(day(10, 3, 9)); err != nil {
		t.Fatal(err)
	}
	onTime := NewReport("J-2", day(10, 2, 0))
	if err := onTime.Submit(day(10, 2, 18)); err != nil {
		t.Fatal(err)
	}
	unsubmitted := NewReport("J-3", day(10, 1, 0))

	// 输入顺序与台账排序不同，按 JobID 和日期定位行。
	rows := Register([]Report{*onTime, *unsubmitted, *late})

	tests := []struct {
		name, jobID, date string
		want              int
	}{
		{"AC-1 迟交两天", "J-1", "2026-10-01", 2},
		{"AC-2 当天提交", "J-2", "2026-10-02", 0},
		{"AC-3 未提交", "J-3", "2026-10-01", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, row := range rows {
				if row.JobID == tt.jobID && row.BusinessDate == tt.date {
					if row.DaysLate != tt.want {
						t.Errorf("DaysLate = %d, want %d", row.DaysLate, tt.want)
					}
					return
				}
			}
			t.Fatalf("row %s %s not found in %+v", tt.jobID, tt.date, rows)
		})
	}
}

func TestRegisterEmpty(t *testing.T) {
	if rows := Register(nil); len(rows) != 0 {
		t.Fatalf("Register(nil) = %v, want empty", rows)
	}
}

func TestRegisterDoesNotReorderInput(t *testing.T) {
	in := []Report{*NewReport("B", day(10, 2, 0)), *NewReport("A", day(10, 1, 0))}
	_ = Register(in)
	if in[0].JobID != "B" || in[1].JobID != "A" {
		t.Fatal("Register must not mutate its input slice")
	}
}
