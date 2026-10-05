package daily

import "testing"

func TestRegisterTotalHours(t *testing.T) {
	a := NewReport("J1", day(10, 1, 0))
	if err := a.Update(map[string]float64{"甲": 8, "乙": 6}); err != nil {
		t.Fatal(err)
	}
	b := NewReport("J2", day(10, 2, 0))
	if err := b.Update(map[string]float64{"丙": 4}); err != nil {
		t.Fatal(err)
	}

	rows := Register([]Report{*a, *b})
	tests := []struct {
		name string
		idx  int
		want float64
	}{
		{"14 小时", 0, a.TotalHours()},
		{"4 小时", 1, b.TotalHours()},
	}
	if len(rows) != len(tests) {
		t.Fatalf("len(rows) = %d, want %d", len(rows), len(tests))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rows[tt.idx].TotalHours; got != tt.want {
				t.Errorf("TotalHours = %v, want %v", got, tt.want)
			}
		})
	}
	if rows[0].TotalHours != 14 || rows[1].TotalHours != 4 {
		t.Errorf("unexpected totals: %v, %v", rows[0].TotalHours, rows[1].TotalHours)
	}
}

func TestHoursByJob(t *testing.T) {
	mk := func(job string, d int, hours float64, submit, lock bool) Report {
		r := NewReport(job, day(10, d, 0))
		if err := r.Update(map[string]float64{"w": hours}); err != nil {
			t.Fatal(err)
		}
		if submit {
			if err := r.Submit(day(10, d, 17)); err != nil {
				t.Fatal(err)
			}
		}
		if lock {
			r.Lock()
		}
		return *r
	}

	tests := []struct {
		name    string
		reports []Report
		want    map[string]float64
	}{
		{"nil 返回空映射", nil, map[string]float64{}},
		{"空切片返回空映射", []Report{}, map[string]float64{}},
		{
			"已提交与已锁定计入，未提交不计",
			[]Report{
				mk("A", 1, 8, true, false),
				mk("A", 2, 6, true, false),
				mk("A", 3, 5, false, false),
				mk("B", 1, 7, true, true),
			},
			map[string]float64{"A": 14, "B": 7},
		},
		{
			"只有未提交的工地不出现",
			[]Report{mk("C", 1, 3, false, false)},
			map[string]float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HoursByJob(tt.reports)
			if got == nil {
				t.Fatal("HoursByJob returned nil")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}
