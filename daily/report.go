// Package daily 实现施工现场日报（日报）的基本模型与操作。
package daily

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// 日报操作可能返回的错误。
var (
	// ErrAlreadySubmitted 表示日报已经提交过，不能重复提交。
	ErrAlreadySubmitted = errors.New("daily: report already submitted")
	// ErrNegativeHours 表示工时为负数。
	ErrNegativeHours = errors.New("daily: crew hours must not be negative")
	// ErrEmptyNote 表示备注内容为空。
	ErrEmptyNote = errors.New("daily: note must not be empty")
	// ErrNoteTooLong 表示备注超过 MaxNoteRunes 个字符。
	ErrNoteTooLong = errors.New("daily: note is too long")
)

// MaxNoteRunes 是单条备注允许的最大字符（rune）数。
const MaxNoteRunes = 500

// Report 是某个工地（JobID）在某个业务日期的一份日报。
type Report struct {
	// JobID 是工地/项目编号。
	JobID string
	// BusinessDate 是日报所属的业务日期，只有日期部分有意义。
	BusinessDate time.Time
	// SubmittedAt 是提交时间；零值表示尚未提交。
	SubmittedAt time.Time
	// Locked 表示日报已被锁定（例如已经结算）。
	Locked bool
	// Crew 记录每个工人当天的工时（小时）。
	Crew map[string]float64
	// Notes 是按时间顺序追加的备注。
	Notes []string
}

// NewReport 创建一份尚未提交的日报，业务日期会被截断到当天零点。
func NewReport(jobID string, businessDate time.Time) *Report {
	return &Report{
		JobID:        jobID,
		BusinessDate: dateOnly(businessDate),
		Crew:         map[string]float64{},
	}
}

// Submitted 报告日报是否已提交。
func (r *Report) Submitted() bool {
	return !r.SubmittedAt.IsZero()
}

// Submit 在时间 at 提交日报；重复提交返回 ErrAlreadySubmitted。
func (r *Report) Submit(at time.Time) error {
	if r.Submitted() {
		return ErrAlreadySubmitted
	}
	r.SubmittedAt = at
	return nil
}

// Lock 锁定日报。
func (r *Report) Lock() {
	r.Locked = true
}

// Update 用 crew 中的工时覆盖对应工人的工时；工时为负数时返回 ErrNegativeHours，且不做任何修改。
func (r *Report) Update(crew map[string]float64) error {
	for name, h := range crew {
		if h < 0 {
			return fmt.Errorf("%w: %s=%v", ErrNegativeHours, name, h)
		}
	}
	if r.Crew == nil {
		r.Crew = map[string]float64{}
	}
	for name, h := range crew {
		r.Crew[name] = h
	}
	return nil
}

// AppendNote 在日报末尾追加一条备注；空白备注返回 ErrEmptyNote，
// 超过 MaxNoteRunes 个字符（rune）返回 ErrNoteTooLong。
func (r *Report) AppendNote(note string) error {
	if strings.TrimSpace(note) == "" {
		return ErrEmptyNote
	}
	if utf8.RuneCountInString(note) > MaxNoteRunes {
		return ErrNoteTooLong
	}
	r.Notes = append(r.Notes, note)
	return nil
}

// LatestNotes 按时间倒序（最新的在前）返回最近 n 条备注；n 大于总数时返回全部。
// 不改变 Notes 的存储顺序。
func (r *Report) LatestNotes(n int) []string {
	if n < 0 {
		n = 0
	}
	if n > len(r.Notes) {
		n = len(r.Notes)
	}
	out := make([]string, 0, n)
	for i := len(r.Notes) - 1; i >= len(r.Notes)-n; i-- {
		out = append(out, r.Notes[i])
	}
	return out
}

// TotalHours 返回全部工人的工时合计。
func (r *Report) TotalHours() float64 {
	var sum float64
	for _, h := range r.Crew {
		sum += h
	}
	return sum
}

// IsLate 报告日报是否迟交：在业务日期当天结束之后提交即视为迟交。
// 未提交的日报不算迟交。
func (r *Report) IsLate() bool {
	return r.DaysLate() > 0
}

// DaysLate 返回提交日期比业务日期晚了多少个自然日；未提交或按时提交时为 0。
func (r *Report) DaysLate() int {
	if !r.Submitted() {
		return 0
	}
	d := daysBetween(r.BusinessDate, r.SubmittedAt)
	if d < 0 {
		return 0
	}
	return d
}

// dateOnly 把 t 截断到其所在时区当天的零点。
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// daysBetween 返回从 from 所在日期到 to 所在日期相差的自然日数。
func daysBetween(from, to time.Time) int {
	a := dateOnly(from)
	b := dateOnly(to.In(from.Location()))
	fy, fm, fd := a.Date()
	ty, tm, td := b.Date()
	// 用 UTC 计算，避免夏令时导致一天不是 24 小时。
	ua := time.Date(fy, fm, fd, 0, 0, 0, 0, time.UTC)
	ub := time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC)
	return int(ub.Sub(ua).Hours() / 24)
}
