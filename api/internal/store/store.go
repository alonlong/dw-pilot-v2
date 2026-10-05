// Package store 负责租户 schema 里日报的读写：在 SQL 行与 daily.Report 之间转换。
//
// 这里只做存取，不做业务判断（迟交、合计、状态、备注校验都在 daily 包）。
// 所有方法都在 db.InTenant 打开的事务里执行，表名不带 schema。
package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/dailylog/api/internal/db"
	"example.com/dailylog/daily"
)

// 存取错误。
var (
	// ErrNotFound 表示日报不存在（含其他租户的日报：它们不在当前 search_path 里）。
	ErrNotFound = errors.New("store: not found")
	// ErrExists 表示同一工地同一天的日报已存在。
	ErrExists = errors.New("store: report already exists")
	// ErrNoJob 表示工地不存在。
	ErrNoJob = errors.New("store: job does not exist")
)

// DateLayout 是业务日期的格式。
const DateLayout = "2006-01-02"

// Job 是工地。
type Job struct {
	ID      string
	Name    string
	Address string
}

// Note 是一条备注。
type Note struct {
	ID         int64
	Body       string
	AuthorName string
	CreatedAt  time.Time
}

// Record 是一份日报及其存储信息。Report.Notes 与 Notes 一一对应，按 id 升序。
type Record struct {
	ID      int64
	JobName string
	Report  daily.Report
	Notes   []Note
}

// Filter 是台账的筛选条件；零值表示不限。From、To 为闭区间的业务日期。
type Filter struct {
	JobID string
	From  time.Time
	To    time.Time
}

// Store 是一个租户事务上的日报存取。
type Store struct {
	q   db.Querier
	loc *time.Location
}

// New 返回事务 q 上的 Store。业务日期按时区 loc 转成 time.Time，
// daily.Report.DaysLate 据此按该时区的自然日计算迟交天数。
func New(q db.Querier, loc *time.Location) *Store {
	return &Store{q: q, loc: loc}
}

// Date 把业务日期 "2006-01-02" 解析为时区 loc 的当天零点。
func Date(s string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(DateLayout, s, loc)
}

// Jobs 返回全部工地，按 id 升序。
func (s *Store) Jobs(ctx context.Context) ([]Job, error) {
	rows, err := s.q.Query(ctx, `select id, name, address from job order by id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Job, error) {
		var j Job
		err := r.Scan(&j.ID, &j.Name, &j.Address)
		return j, err
	})
}

// Reports 返回符合条件的日报（含工时，不含备注），按 id 升序。
func (s *Store) Reports(ctx context.Context, f Filter) ([]Record, error) {
	where := "true"
	var args []any
	if f.JobID != "" {
		args = append(args, f.JobID)
		where += fmt.Sprintf(" and r.job_id = $%d", len(args))
	}
	if !f.From.IsZero() {
		args = append(args, f.From.Format(DateLayout))
		where += fmt.Sprintf(" and r.business_date >= $%d::date", len(args))
	}
	if !f.To.IsZero() {
		args = append(args, f.To.Format(DateLayout))
		where += fmt.Sprintf(" and r.business_date <= $%d::date", len(args))
	}
	rows, err := s.q.Query(ctx, `select r.id, r.job_id, j.name, r.business_date, r.submitted_at, r.locked
		from report r join job j on j.id = r.job_id
		where `+where+` order by r.id`, args...)
	if err != nil {
		return nil, err
	}
	recs, err := pgx.CollectRows(rows, s.scanRecord)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return recs, nil
	}
	byID := make(map[int64]*Record, len(recs))
	ids := make([]int64, len(recs))
	for i := range recs {
		byID[recs[i].ID] = &recs[i]
		ids[i] = recs[i].ID
	}
	crew, err := s.q.Query(ctx, `select report_id, worker, hours from crew_hours where report_id = any($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer crew.Close()
	for crew.Next() {
		var id int64
		var worker string
		var hours float64
		if err := crew.Scan(&id, &worker, &hours); err != nil {
			return nil, err
		}
		byID[id].Report.Crew[worker] = hours
	}
	return recs, crew.Err()
}

// Report 返回一份日报（含工时和备注）。forUpdate 为真时锁住该行直到事务结束，
// 用于先读后改的操作，避免两个并发请求都基于旧状态修改。
func (s *Store) Report(ctx context.Context, id int64, forUpdate bool) (*Record, error) {
	sql := `select r.id, r.job_id, j.name, r.business_date, r.submitted_at, r.locked
		from report r join job j on j.id = r.job_id where r.id = $1`
	if forUpdate {
		sql += ` for update of r`
	}
	rows, err := s.q.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}
	rec, err := pgx.CollectExactlyOneRow(rows, s.scanRecord)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	crew, err := s.q.Query(ctx, `select worker, hours from crew_hours where report_id = $1`, id)
	if err != nil {
		return nil, err
	}
	for crew.Next() {
		var worker string
		var hours float64
		if err := crew.Scan(&worker, &hours); err != nil {
			crew.Close()
			return nil, err
		}
		rec.Report.Crew[worker] = hours
	}
	if err := crew.Err(); err != nil {
		return nil, err
	}

	notes, err := s.q.Query(ctx, `select id, body, author_name, created_at from report_note where report_id = $1 order by id`, id)
	if err != nil {
		return nil, err
	}
	rec.Notes, err = pgx.CollectRows(notes, func(r pgx.CollectableRow) (Note, error) {
		var n Note
		err := r.Scan(&n.ID, &n.Body, &n.AuthorName, &n.CreatedAt)
		return n, err
	})
	if err != nil {
		return nil, err
	}
	for _, n := range rec.Notes {
		rec.Report.Notes = append(rec.Report.Notes, n.Body)
	}
	return &rec, nil
}

func (s *Store) scanRecord(row pgx.CollectableRow) (Record, error) {
	var rec Record
	var jobID string
	var date time.Time
	var submitted *time.Time
	var locked bool
	if err := row.Scan(&rec.ID, &jobID, &rec.JobName, &date, &submitted, &locked); err != nil {
		return Record{}, err
	}
	// date 列读出来是 UTC 零点，只取年月日，放到业务时区。
	y, m, d := date.Date()
	rec.Report = *daily.NewReport(jobID, time.Date(y, m, d, 0, 0, 0, 0, s.loc))
	if submitted != nil {
		rec.Report.SubmittedAt = *submitted
	}
	rec.Report.Locked = locked
	return rec, nil
}

// Create 新建一份空日报，返回 id。工地不存在返回 ErrNoJob，已存在返回 ErrExists。
func (s *Store) Create(ctx context.Context, r *daily.Report, now time.Time) (int64, error) {
	var exists bool
	if err := s.q.QueryRow(ctx, `select exists (select 1 from job where id = $1)`, r.JobID).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrNoJob
	}
	var id int64
	err := s.q.QueryRow(ctx, `insert into report (job_id, business_date, created_at) values ($1, $2::date, $3)
		on conflict (job_id, business_date) do nothing returning id`,
		r.JobID, r.BusinessDate.Format(DateLayout), now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrExists
	}
	return id, err
}

// SaveCrew 用 crew 整体替换日报的工时。
func (s *Store) SaveCrew(ctx context.Context, id int64, crew map[string]float64) error {
	if _, err := s.q.Exec(ctx, `delete from crew_hours where report_id = $1`, id); err != nil {
		return err
	}
	workers := make([]string, 0, len(crew))
	for w := range crew {
		workers = append(workers, w)
	}
	sort.Strings(workers)
	for _, w := range workers {
		if _, err := s.q.Exec(ctx, `insert into crew_hours (report_id, worker, hours) values ($1, $2, $3)`, id, w, crew[w]); err != nil {
			return err
		}
	}
	return nil
}

// SaveState 保存日报的提交时间和锁定状态。
func (s *Store) SaveState(ctx context.Context, id int64, r *daily.Report) error {
	var submitted *time.Time
	if r.Submitted() {
		t := r.SubmittedAt.UTC()
		submitted = &t
	}
	tag, err := s.q.Exec(ctx, `update report set submitted_at = $2, locked = $3 where id = $1`, id, submitted, r.Locked)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddNote 追加一条备注；authorID 为 0 表示没有对应的用户。
func (s *Store) AddNote(ctx context.Context, id int64, body string, authorID int64, authorName string, now time.Time) error {
	var author *int64
	if authorID > 0 {
		author = &authorID
	}
	_, err := s.q.Exec(ctx, `insert into report_note (report_id, body, author_id, author_name, created_at) values ($1, $2, $3, $4, $5)`,
		id, body, author, authorName, now)
	return err
}
