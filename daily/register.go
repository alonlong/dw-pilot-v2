package daily

import (
	"sort"
)

// 台账中日报的状态文本。
const (
	StatusDraft     = "未提交"
	StatusSubmitted = "已提交"
	StatusLocked    = "已锁定"
)

// Row 是日报台账（列表）中的一行。
type Row struct {
	// JobID 是工地/项目编号。
	JobID string
	// BusinessDate 是业务日期，格式为 "2006-01-02"。
	BusinessDate string
	// Status 是状态文本，取值见 StatusDraft 等常量。
	Status string
	// TotalHours 是该日报全部工人的工时合计。
	TotalHours float64
	// DaysLate 是迟交的自然日数，未提交或按时提交为 0。
	DaysLate int
}

// HoursByJob 按工地汇总已提交（含已锁定）日报的总工时，未提交的日报不计入。
// 没有符合条件的日报时返回空映射而不是 nil。
func HoursByJob(reports []Report) map[string]float64 {
	out := map[string]float64{}
	for i := range reports {
		r := &reports[i]
		if r.Submitted() || r.Locked {
			out[r.JobID] += r.TotalHours()
		}
	}
	return out
}

// LateCountByJob 按工地统计迟交（IsLate 为真）的日报份数，未提交和按时的日报不计入。
// 没有迟交日报的工地不出现在结果中；没有符合条件的日报时返回空映射而不是 nil。
func LateCountByJob(reports []Report) map[string]int {
	out := map[string]int{}
	for i := range reports {
		r := &reports[i]
		if r.IsLate() {
			out[r.JobID]++
		}
	}
	return out
}

// Register 把日报转换为台账行，按业务日期升序排列，同一天再按 JobID 升序。
func Register(reports []Report) []Row {
	sorted := make([]Report, len(reports))
	copy(sorted, reports)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if !a.BusinessDate.Equal(b.BusinessDate) {
			return a.BusinessDate.Before(b.BusinessDate)
		}
		return a.JobID < b.JobID
	})

	rows := make([]Row, 0, len(sorted))
	for _, r := range sorted {
		rows = append(rows, Row{
			JobID:        r.JobID,
			BusinessDate: r.BusinessDate.Format("2006-01-02"),
			Status:       status(&r),
			TotalHours:   r.TotalHours(),
			DaysLate:     r.DaysLate(),
		})
	}
	return rows
}

// status 返回日报在台账中的状态文本。
func status(r *Report) string {
	switch {
	case r.Locked:
		return StatusLocked
	case r.Submitted():
		return StatusSubmitted
	default:
		return StatusDraft
	}
}
