package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"example.com/dailylog/api/internal/auth"
	"example.com/dailylog/api/internal/store"
	"example.com/dailylog/daily"
)

// 工时表的输入限制（daily 包不限制，这里只防止异常输入）。
const (
	maxWorkerRunes = 50
	maxCrewEntries = 100
)

// ---- JSON 形状（docs/api.md） ----

type meJSON struct {
	ID         int64     `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       auth.Role `json:"role"`
	Tenant     string    `json:"tenant"`
	TenantName string    `json:"tenantName"`
}

type jobJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

type reportRowJSON struct {
	ID           int64   `json:"id"`
	JobID        string  `json:"jobId"`
	JobName      string  `json:"jobName"`
	BusinessDate string  `json:"businessDate"`
	Status       string  `json:"status"`
	TotalHours   float64 `json:"totalHours"`
	DaysLate     int     `json:"daysLate"`
	NoteCount    int     `json:"noteCount"`
}

type crewEntryJSON struct {
	Worker string  `json:"worker"`
	Hours  float64 `json:"hours"`
}

type noteJSON struct {
	ID         int64  `json:"id"`
	Body       string `json:"body"`
	AuthorName string `json:"authorName"`
	CreatedAt  string `json:"createdAt"`
}

type canJSON struct {
	EditCrew bool `json:"editCrew"`
	Submit   bool `json:"submit"`
	Lock     bool `json:"lock"`
	AddNote  bool `json:"addNote"`
}

type reportDetailJSON struct {
	ID           int64           `json:"id"`
	JobID        string          `json:"jobId"`
	JobName      string          `json:"jobName"`
	BusinessDate string          `json:"businessDate"`
	Status       string          `json:"status"`
	SubmittedAt  *string         `json:"submittedAt"`
	Locked       bool            `json:"locked"`
	TotalHours   float64         `json:"totalHours"`
	DaysLate     int             `json:"daysLate"`
	Crew         []crewEntryJSON `json:"crew"`
	Notes        []noteJSON      `json:"notes"`
	Can          canJSON         `json:"can"`
}

type summaryRowJSON struct {
	JobID      string  `json:"jobId"`
	JobName    string  `json:"jobName"`
	TotalHours float64 `json:"totalHours"`
	LateCount  int     `json:"lateCount"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func toMe(u *auth.User) meJSON {
	return meJSON{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role, Tenant: u.Tenant, TenantName: u.TenantName}
}

// detail 把日报转成 ReportDetail；状态、合计、迟交天数、备注顺序都来自 daily 包。
func detail(rec *store.Record, role auth.Role) reportDetailJSON {
	r := &rec.Report
	row := daily.Register([]daily.Report{*r})[0]
	d := reportDetailJSON{
		ID:           rec.ID,
		JobID:        r.JobID,
		JobName:      rec.JobName,
		BusinessDate: row.BusinessDate,
		Status:       row.Status,
		Locked:       r.Locked,
		TotalHours:   r.TotalHours(),
		DaysLate:     r.DaysLate(),
		Crew:         []crewEntryJSON{},
		Notes:        []noteJSON{},
		Can: canJSON{
			EditCrew: role.Can(auth.EditReport) && !r.Locked,
			Submit:   role.Can(auth.EditReport) && !r.Locked && !r.Submitted(),
			Lock:     role.Can(auth.Lock) && !r.Locked,
			AddNote:  role.Can(auth.AddNote),
		},
	}
	if r.Submitted() {
		at := formatTime(r.SubmittedAt)
		d.SubmittedAt = &at
	}
	workers := make([]string, 0, len(r.Crew))
	for w := range r.Crew {
		workers = append(workers, w)
	}
	sort.Strings(workers)
	for _, w := range workers {
		d.Crew = append(d.Crew, crewEntryJSON{Worker: w, Hours: r.Crew[w]})
	}
	// LatestNotes 给出新的在前的正文；rec.Notes 按 id 升序，与 r.Notes 一一对应。
	for i, body := range r.LatestNotes(len(r.Notes)) {
		n := rec.Notes[len(rec.Notes)-1-i]
		d.Notes = append(d.Notes, noteJSON{ID: n.ID, Body: body, AuthorName: n.AuthorName, CreatedAt: formatTime(n.CreatedAt)})
	}
	return d
}

// dailyError 把 daily 包的错误转成接口错误。
func dailyError(err error) error {
	switch {
	case errors.Is(err, daily.ErrAlreadySubmitted):
		return newError(http.StatusConflict, "already_submitted", "日报已提交，不能重复提交")
	case errors.Is(err, daily.ErrNegativeHours):
		msg := "工时不能为负数"
		if detail := strings.TrimPrefix(err.Error(), daily.ErrNegativeHours.Error()+": "); detail != err.Error() {
			msg += "：" + detail
		}
		return newError(http.StatusUnprocessableEntity, "negative_hours", msg)
	case errors.Is(err, daily.ErrEmptyNote):
		return newError(http.StatusUnprocessableEntity, "empty_note", "备注不能为空")
	case errors.Is(err, daily.ErrNoteTooLong):
		return newError(http.StatusUnprocessableEntity, "note_too_long", fmt.Sprintf("备注不能超过 %d 个字符", daily.MaxNoteRunes))
	}
	return err
}

func storeError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return errNotFound
	case errors.Is(err, store.ErrExists):
		return newError(http.StatusConflict, "already_exists", "该工地当天的日报已存在")
	case errors.Is(err, store.ErrNoJob):
		return badRequest("工地不存在")
	}
	return err
}

// decode 读取 JSON 请求体到 v。
func decode(c echo.Context, v any) error {
	dec := json.NewDecoder(c.Request().Body)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return badRequest("请求体为空")
		}
		// 超过 BodyLimit 时读取请求体返回 Echo 的 413 错误。
		var he *echo.HTTPError
		if errors.As(err, &he) {
			return he
		}
		return badRequest("请求体不是有效的 JSON")
	}
	return nil
}

func reportID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound
	}
	return id, nil
}

// dateParam 解析可选的日期查询参数。
func (s *server) dateParam(c echo.Context, name string) (time.Time, error) {
	v := c.QueryParam(name)
	if v == "" {
		return time.Time{}, nil
	}
	t, err := store.Date(v, s.Location)
	if err != nil {
		return time.Time{}, badRequest(fmt.Sprintf("参数 %s 不是 YYYY-MM-DD 格式的日期", name))
	}
	return t, nil
}

// ---- 处理函数 ----

func (s *server) healthz(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		s.Log.Error("数据库不通", "err", err)
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "db_unavailable"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) version(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"sha": s.BuildSHA})
}

func (s *server) login(c echo.Context) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	ctx := c.Request().Context()
	u, err := auth.Login(ctx, s.Pool, in.Email, in.Password)
	if errors.Is(err, auth.ErrBadCredentials) {
		return errBadCredentials
	}
	if err != nil {
		return err
	}
	token, expires, err := auth.NewSession(ctx, s.Pool, u.ID, s.Now())
	if err != nil {
		return err
	}
	s.setCookie(c, token, expires)
	return c.JSON(http.StatusOK, toMe(u))
}

// logout 删除会话并清除 Cookie；没有会话时也返回 204，前端可以无条件调用。
func (s *server) logout(c echo.Context) error {
	if ck, err := c.Cookie(CookieName); err == nil {
		if err := auth.DeleteSession(c.Request().Context(), s.Pool, ck.Value); err != nil {
			return err
		}
	}
	s.clearCookie(c)
	return c.NoContent(http.StatusNoContent)
}

func (s *server) me(c echo.Context) error {
	return c.JSON(http.StatusOK, toMe(currentUser(c)))
}

func (s *server) listJobs(c echo.Context) error {
	var jobs []store.Job
	err := s.inTenant(c, func(st *store.Store) error {
		var err error
		jobs, err = st.Jobs(c.Request().Context())
		return err
	})
	if err != nil {
		return err
	}
	out := make([]jobJSON, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, jobJSON{ID: j.ID, Name: j.Name, Address: j.Address})
	}
	return c.JSON(http.StatusOK, map[string]any{"jobs": out})
}

func (s *server) listReports(c echo.Context) error {
	from, err := s.dateParam(c, "from")
	if err != nil {
		return err
	}
	to, err := s.dateParam(c, "to")
	if err != nil {
		return err
	}
	var recs []store.Record
	err = s.inTenant(c, func(st *store.Store) error {
		var err error
		recs, err = st.Reports(c.Request().Context(), store.Filter{JobID: c.QueryParam("job"), From: from, To: to})
		return err
	})
	if err != nil {
		return err
	}
	// daily.Register 给出排序和各列；同一工地同一天只有一份日报，按 (工地, 日期) 找回 id 和工地名。
	type key struct{ job, date string }
	byKey := make(map[key]*store.Record, len(recs))
	reports := make([]daily.Report, len(recs))
	for i := range recs {
		reports[i] = recs[i].Report
		byKey[key{recs[i].Report.JobID, recs[i].Report.BusinessDate.Format(store.DateLayout)}] = &recs[i]
	}
	out := make([]reportRowJSON, 0, len(recs))
	for _, row := range daily.Register(reports) {
		rec := byKey[key{row.JobID, row.BusinessDate}]
		out = append(out, reportRowJSON{
			ID:           rec.ID,
			JobID:        row.JobID,
			JobName:      rec.JobName,
			BusinessDate: row.BusinessDate,
			Status:       row.Status,
			TotalHours:   row.TotalHours,
			DaysLate:     row.DaysLate,
			NoteCount:    rec.NoteCount,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": out})
}

func (s *server) createReport(c echo.Context) error {
	var in struct {
		JobID        string `json:"jobId"`
		BusinessDate string `json:"businessDate"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	in.JobID = strings.TrimSpace(in.JobID)
	if in.JobID == "" {
		return badRequest("缺少 jobId")
	}
	date, err := store.Date(in.BusinessDate, s.Location)
	if err != nil {
		return badRequest("businessDate 不是 YYYY-MM-DD 格式的日期")
	}
	var out reportDetailJSON
	err = s.inTenant(c, func(st *store.Store) error {
		ctx := c.Request().Context()
		id, err := st.Create(ctx, daily.NewReport(in.JobID, date), s.Now())
		if err != nil {
			return storeError(err)
		}
		return s.reload(c, st, id, &out)
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, out)
}

func (s *server) getReport(c echo.Context) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var out reportDetailJSON
	err = s.inTenant(c, func(st *store.Store) error {
		return s.reload(c, st, id, &out)
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

// reload 重新读取日报并转成 ReportDetail。
func (s *server) reload(c echo.Context, st *store.Store, id int64, out *reportDetailJSON) error {
	rec, err := st.Report(c.Request().Context(), id, false)
	if err != nil {
		return storeError(err)
	}
	*out = detail(rec, currentUser(c).Role)
	return nil
}

// modify 锁住日报后执行 fn，再返回修改后的 ReportDetail。
func (s *server) modify(c echo.Context, status int, fn func(ctx context.Context, st *store.Store, rec *store.Record) error) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var out reportDetailJSON
	err = s.inTenant(c, func(st *store.Store) error {
		ctx := c.Request().Context()
		rec, err := st.Report(ctx, id, true)
		if err != nil {
			return storeError(err)
		}
		if err := fn(ctx, st, rec); err != nil {
			return err
		}
		return s.reload(c, st, id, &out)
	})
	if err != nil {
		return err
	}
	return c.JSON(status, out)
}

func (s *server) saveCrew(c echo.Context) error {
	var in struct {
		Crew []crewEntryJSON `json:"crew"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	if in.Crew == nil {
		return badRequest("缺少 crew")
	}
	if len(in.Crew) > maxCrewEntries {
		return badRequest(fmt.Sprintf("工人不能超过 %d 个", maxCrewEntries))
	}
	crew := make(map[string]float64, len(in.Crew))
	for _, e := range in.Crew {
		w := strings.TrimSpace(e.Worker)
		if w == "" {
			return badRequest("工人姓名不能为空")
		}
		if utf8.RuneCountInString(w) > maxWorkerRunes {
			return badRequest(fmt.Sprintf("工人姓名不能超过 %d 个字符", maxWorkerRunes))
		}
		if _, dup := crew[w]; dup {
			return badRequest("工人重复：" + w)
		}
		crew[w] = e.Hours
	}
	return s.modify(c, http.StatusOK, func(ctx context.Context, st *store.Store, rec *store.Record) error {
		if rec.Report.Locked {
			return errLocked
		}
		// 整体替换：从空工时表开始用 daily 的 Update 写入（负数校验在 Update 里）。
		r := rec.Report
		r.Crew = map[string]float64{}
		if err := r.Update(crew); err != nil {
			return dailyError(err)
		}
		return st.SaveCrew(ctx, rec.ID, r.Crew)
	})
}

func (s *server) submit(c echo.Context) error {
	return s.modify(c, http.StatusOK, func(ctx context.Context, st *store.Store, rec *store.Record) error {
		if rec.Report.Locked {
			return errLocked
		}
		if err := rec.Report.Submit(s.Now()); err != nil {
			return dailyError(err)
		}
		return st.SaveState(ctx, rec.ID, &rec.Report)
	})
}

func (s *server) lock(c echo.Context) error {
	return s.modify(c, http.StatusOK, func(ctx context.Context, st *store.Store, rec *store.Record) error {
		if rec.Report.Locked {
			return newError(http.StatusConflict, "locked", "日报已锁定")
		}
		rec.Report.Lock()
		return st.SaveState(ctx, rec.ID, &rec.Report)
	})
}

func (s *server) addNote(c echo.Context) error {
	var in struct {
		Body string `json:"body"`
	}
	if err := decode(c, &in); err != nil {
		return err
	}
	u := currentUser(c)
	return s.modify(c, http.StatusCreated, func(ctx context.Context, st *store.Store, rec *store.Record) error {
		// 锁定不影响追加备注；校验在 daily 的 AppendNote 里。
		if err := rec.Report.AppendNote(in.Body); err != nil {
			return dailyError(err)
		}
		return st.AddNote(ctx, rec.ID, in.Body, u.ID, u.Name, s.Now())
	})
}

func (s *server) summary(c echo.Context) error {
	from, err := s.dateParam(c, "from")
	if err != nil {
		return err
	}
	to, err := s.dateParam(c, "to")
	if err != nil {
		return err
	}
	var recs []store.Record
	err = s.inTenant(c, func(st *store.Store) error {
		var err error
		recs, err = st.Reports(c.Request().Context(), store.Filter{From: from, To: to})
		return err
	})
	if err != nil {
		return err
	}
	reports := make([]daily.Report, len(recs))
	names := map[string]string{}
	for i := range recs {
		reports[i] = recs[i].Report
		names[recs[i].Report.JobID] = recs[i].JobName
	}
	hours := daily.HoursByJob(reports)
	late := daily.LateCountByJob(reports)
	ids := make([]string, 0, len(hours)+len(late))
	for id := range hours {
		ids = append(ids, id)
	}
	for id := range late {
		if _, ok := hours[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := make([]summaryRowJSON, 0, len(ids))
	for _, id := range ids {
		out = append(out, summaryRowJSON{JobID: id, JobName: names[id], TotalHours: hours[id], LateCount: late[id]})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": out})
}
