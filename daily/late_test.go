package daily

import "testing"

func TestLateCountByJob(t *testing.T) {
	// mk 构造日报；submitDay 为 0 表示未提交。
	mk := func(job string, bizDay, submitDay, submitHour int) Report {
		r := NewReport(job, day(10, bizDay, 0))
		if submitDay > 0 {
			if err := r.Submit(day(10, submitDay, submitHour)); err != nil {
				t.Fatal(err)
			}
		}
		return *r
	}

	tests := []struct {
		name    string
		reports []Report
		want    map[string]int
	}{
		{"nil 输入返回空映射", nil, map[string]int{}},
		{"空切片输入返回空映射", []Report{}, map[string]int{}},
		{
			"J-1 两份迟交加一份按时",
			[]Report{
				mk("J-1", 1, 3, 9),
				mk("J-1", 2, 4, 9),
				mk("J-1", 5, 5, 23),
			},
			map[string]int{"J-1": 2},
		},
		{
			"J-2 仅有未提交日报不出现",
			[]Report{
				mk("J-2", 1, 0, 0),
				mk("J-1", 1, 3, 9),
			},
			map[string]int{"J-1": 1},
		},
		{
			"只有按时日报的工地不出现",
			[]Report{mk("J-3", 1, 1, 17)},
			map[string]int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LateCountByJob(tt.reports)
			if got == nil {
				t.Fatal("LateCountByJob returned nil")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%s] = %d, want %d", k, got[k], v)
				}
			}
			if _, ok := got["J-2"]; ok {
				t.Errorf("J-2 should not be in result: %v", got)
			}
		})
	}
}
