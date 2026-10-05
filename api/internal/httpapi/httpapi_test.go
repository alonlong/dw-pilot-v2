package httpapi_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"example.com/dailylog/api/internal/httpapi"
	"example.com/dailylog/api/internal/testdb"
)

func TestPublicAndUnauthenticated(t *testing.T) {
	pool := testdb.Pool(t)
	v := newEnv(t, pool)

	rec := v.do(http.MethodGet, "/healthz", nil, nil)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Errorf("/healthz = %d %s", rec.Code, rec.Body)
	}
	rec = v.do(http.MethodGet, "/api/version", nil, nil)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"sha":"test-sha"}` {
		t.Errorf("/api/version = %d %s", rec.Code, rec.Body)
	}

	tests := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/me"},
		{http.MethodGet, "/api/jobs"},
		{http.MethodGet, "/api/reports"},
		{http.MethodPost, "/api/reports"},
		{http.MethodGet, "/api/reports/1"},
		{http.MethodPut, "/api/reports/1/crew"},
		{http.MethodPost, "/api/reports/1/submit"},
		{http.MethodPost, "/api/reports/1/lock"},
		{http.MethodPost, "/api/reports/1/notes"},
		{http.MethodGet, "/api/summary"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := v.do(tt.method, tt.path, nil, nil)
			if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthenticated" {
				t.Errorf("= %d %s", rec.Code, rec.Body)
			}
			// 伪造的 Cookie 也一样。
			rec = v.do(tt.method, tt.path, nil, &http.Cookie{Name: httpapi.CookieName, Value: "forged"})
			if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthenticated" {
				t.Errorf("伪造 Cookie = %d %s", rec.Code, rec.Body)
			}
		})
	}

	t.Run("未知的 /api 路径", func(t *testing.T) {
		rec := v.do(http.MethodGet, "/api/nope", nil, nil)
		if rec.Code != http.StatusNotFound || errCode(t, rec) != "not_found" {
			t.Errorf("= %d %s", rec.Code, rec.Body)
		}
	})
}

func TestLogin(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)

	tests := []struct {
		name       string
		secure     bool
		body       any
		wantStatus int
		wantCode   string
	}{
		{"成功", false, map[string]string{"email": "manager@" + slotA + ".test", "password": testdb.Password}, 200, ""},
		{"成功（Secure）", true, map[string]string{"email": "manager@" + slotA + ".test", "password": testdb.Password}, 200, ""},
		{"密码错误", false, map[string]string{"email": "manager@" + slotA + ".test", "password": "wrong"}, 401, "bad_credentials"},
		{"邮箱不存在", false, map[string]string{"email": "who@" + slotA + ".test", "password": testdb.Password}, 401, "bad_credentials"},
		{"请求体不是 JSON", false, "{", 400, "bad_request"},
		{"没有请求体", false, nil, 400, "bad_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newEnv(t, pool, func(o *httpapi.Options) { o.CookieSecure = tt.secure })
			rec := v.do(http.MethodPost, "/api/login", tt.body, nil)
			if rec.Code != tt.wantStatus {
				t.Fatalf("= %d %s", rec.Code, rec.Body)
			}
			if tt.wantCode != "" {
				if got := errCode(t, rec); got != tt.wantCode {
					t.Errorf("code = %q, want %q", got, tt.wantCode)
				}
				if len(rec.Result().Cookies()) != 0 {
					t.Error("失败的登录不应设置 Cookie")
				}
				return
			}
			me := decodeJSON[map[string]any](t, rec)
			if me["email"] != "manager@"+slotA+".test" || me["role"] != "manager" || me["tenant"] != slotA ||
				me["tenantName"] == "" || me["name"] == "" || me["id"] == nil {
				t.Errorf("Me = %v", me)
			}
			cks := rec.Result().Cookies()
			if len(cks) != 1 {
				t.Fatalf("cookies = %v", cks)
			}
			ck := cks[0]
			if ck.Name != "dl_session" || !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.Path != "/" ||
				ck.MaxAge != 12*3600 || ck.Secure != tt.secure || len(ck.Value) < 40 {
				t.Errorf("cookie = %+v", ck)
			}
		})
	}
}

func TestSessionExpiryAndLogout(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	start := v.now
	ck := v.login("field@" + slotA + ".test")

	tests := []struct {
		name  string
		after time.Duration
		want  int
	}{
		{"刚登录", 0, 200},
		{"11 小时后", 11 * time.Hour, 200},
		{"12 小时后过期", 12 * time.Hour, 401},
		{"13 小时后", 13 * time.Hour, 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v.now = start.Add(tt.after)
			rec := v.do(http.MethodGet, "/api/me", nil, ck)
			if rec.Code != tt.want {
				t.Errorf("= %d %s", rec.Code, rec.Body)
			}
		})
	}

	v.now = start
	ck = v.login("field@" + slotA + ".test")
	rec := v.do(http.MethodPost, "/api/logout", nil, ck)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout = %d %s", rec.Code, rec.Body)
	}
	if cks := rec.Result().Cookies(); len(cks) != 1 || cks[0].MaxAge >= 0 {
		t.Errorf("logout 没有清除 Cookie：%v", cks)
	}
	if rec := v.do(http.MethodGet, "/api/me", nil, ck); rec.Code != http.StatusUnauthorized {
		t.Errorf("登出后 /api/me = %d", rec.Code)
	}
	if rec := v.do(http.MethodPost, "/api/logout", nil, nil); rec.Code != http.StatusNoContent {
		t.Errorf("没有会话时 logout = %d", rec.Code)
	}
}

// TestPermissions 覆盖每个接口、每个角色的允许与拒绝（docs/api.md 的权限表）。
func TestPermissions(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	cks := v.loginAll(slotA)

	type req struct {
		method, path string
		body         any
	}
	get := func(p string) map[string]req {
		return map[string]req{"admin": {"GET", p, nil}, "manager": {"GET", p, nil}, "field": {"GET", p, nil}, "finance": {"GET", p, nil}}
	}
	newReport := func(day int) req {
		return req{"POST", "/api/reports", map[string]string{"jobId": "J-24002", "businessDate": fmt.Sprintf("2026-09-%02d", day)}}
	}
	crew := map[string]any{"crew": []map[string]any{{"worker": "甲", "hours": 8}}}
	note := map[string]string{"body": "备注"}
	// 允许的角色各用不同的日报，避免前一个角色改了状态影响后一个；拒绝的角色在读日报之前就被拒绝。
	tests := []struct {
		name string
		reqs map[string]req
		want map[string]int
	}{
		{"GET /api/me", get("/api/me"), map[string]int{"admin": 200, "manager": 200, "field": 200, "finance": 200}},
		{"GET /api/jobs", get("/api/jobs"), map[string]int{"admin": 200, "manager": 200, "field": 200, "finance": 200}},
		{"GET /api/reports", get("/api/reports"), map[string]int{"admin": 200, "manager": 200, "field": 200, "finance": 200}},
		{"GET /api/reports/:id", get("/api/reports/1"), map[string]int{"admin": 200, "manager": 200, "field": 200, "finance": 200}},
		{"GET /api/summary", get("/api/summary"), map[string]int{"admin": 200, "manager": 200, "field": 403, "finance": 200}},
		{"POST /api/reports", map[string]req{
			"admin": newReport(21), "manager": newReport(22), "field": newReport(23), "finance": newReport(24),
		}, map[string]int{"admin": 201, "manager": 403, "field": 201, "finance": 403}},
		{"PUT /api/reports/:id/crew", map[string]req{
			"admin": {"PUT", "/api/reports/9/crew", crew}, "manager": {"PUT", "/api/reports/9/crew", crew},
			"field": {"PUT", "/api/reports/10/crew", crew}, "finance": {"PUT", "/api/reports/9/crew", crew},
		}, map[string]int{"admin": 200, "manager": 403, "field": 200, "finance": 403}},
		{"POST /api/reports/:id/submit", map[string]req{
			"admin": {"POST", "/api/reports/9/submit", nil}, "manager": {"POST", "/api/reports/20/submit", nil},
			"field": {"POST", "/api/reports/10/submit", nil}, "finance": {"POST", "/api/reports/20/submit", nil},
		}, map[string]int{"admin": 200, "manager": 403, "field": 200, "finance": 403}},
		{"POST /api/reports/:id/lock", map[string]req{
			"admin": {"POST", "/api/reports/3/lock", nil}, "manager": {"POST", "/api/reports/4/lock", nil},
			"field": {"POST", "/api/reports/5/lock", nil}, "finance": {"POST", "/api/reports/5/lock", nil},
		}, map[string]int{"admin": 200, "manager": 200, "field": 403, "finance": 403}},
		{"POST /api/reports/:id/notes", map[string]req{
			"admin": {"POST", "/api/reports/6/notes", note}, "manager": {"POST", "/api/reports/6/notes", note},
			"field": {"POST", "/api/reports/6/notes", note}, "finance": {"POST", "/api/reports/6/notes", note},
		}, map[string]int{"admin": 201, "manager": 201, "field": 201, "finance": 403}},
	}
	for _, tt := range tests {
		for _, role := range []string{"admin", "manager", "field", "finance"} {
			t.Run(tt.name+"/"+role, func(t *testing.T) {
				r := tt.reqs[role]
				rec := v.do(r.method, r.path, r.body, cks[role])
				if rec.Code != tt.want[role] {
					t.Fatalf("= %d, want %d：%s", rec.Code, tt.want[role], rec.Body)
				}
				if rec.Code == 403 {
					if got := errCode(t, rec); got != "forbidden" {
						t.Errorf("code = %q, want forbidden", got)
					}
				}
			})
		}
	}
	// 被拒绝的请求没有留下改动。
	rec := v.do("GET", "/api/reports/20", nil, cks["admin"])
	if d := decodeJSON[detail](t, rec); d.Status != "未提交" {
		t.Errorf("被拒绝的提交改了日报 20：%s", d.Status)
	}
	rec = v.do("GET", "/api/reports/5", nil, cks["admin"])
	if d := decodeJSON[detail](t, rec); d.Locked {
		t.Error("被拒绝的锁定改了日报 5")
	}
}

// TestCan 检查 ReportDetail.can 按角色和日报状态算好。
func TestCan(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	cks := v.loginAll(slotA)
	// 9 = 未提交，4 = 已提交，1 = 已锁定。
	tests := []struct {
		role string
		id   int
		want can
	}{
		{"admin", 9, can{true, true, true, true}},
		{"admin", 4, can{true, false, true, true}},
		{"admin", 1, can{false, false, false, true}},
		{"manager", 9, can{false, false, true, true}},
		{"manager", 1, can{false, false, false, true}},
		{"field", 9, can{true, true, false, true}},
		{"field", 4, can{true, false, false, true}},
		{"field", 1, can{false, false, false, true}},
		{"finance", 9, can{false, false, false, false}},
		{"finance", 1, can{false, false, false, false}},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%d", tt.role, tt.id), func(t *testing.T) {
			rec := v.do("GET", fmt.Sprintf("/api/reports/%d", tt.id), nil, cks[tt.role])
			if got := decodeJSON[detail](t, rec).Can; got != tt.want {
				t.Errorf("can = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestListReports(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	ck := v.login("finance@" + slotA + ".test")

	rec := v.do("GET", "/api/reports", nil, ck)
	rows := decodeJSON[struct {
		Rows []row `json:"rows"`
	}](t, rec).Rows
	if len(rows) != 30 {
		t.Fatalf("行数 = %d, want 30", len(rows))
	}
	// 按业务日期、工地升序：快照 id 的排列是 1, 11, 21, 2, 12, 22, …
	for i, r := range rows {
		day, job := i/3+1, i%3
		if want := int64(job*10 + day); r.ID != want {
			t.Errorf("rows[%d].id = %d, want %d", i, r.ID, want)
		}
	}
	byID := map[int64]row{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	want := []row{
		{1, "J-24001", "滨江示范小学扩建（虚构）", "2026-09-01", "已锁定", 33.5, 0},
		{3, "J-24001", "滨江示范小学扩建（虚构）", "2026-09-03", "已提交", 33.5, 1},
		{5, "J-24001", "滨江示范小学扩建（虚构）", "2026-09-05", "已提交", 33.5, 0},
		{10, "J-24001", "滨江示范小学扩建（虚构）", "2026-09-10", "未提交", 0, 0},
		{12, "J-24002", "青石社区卫生站（虚构）", "2026-09-02", "已提交", 25.5, 2},
		{15, "J-24002", "青石社区卫生站（虚构）", "2026-09-05", "已提交", 25.5, 1},
		{22, "J-24003", "北岸物流仓库（虚构）", "2026-09-02", "已锁定", 41.5, 1},
		{24, "J-24003", "北岸物流仓库（虚构）", "2026-09-04", "已提交", 41.5, 3},
	}
	for _, w := range want {
		if got := byID[w.ID]; got != w {
			t.Errorf("row %d = %+v, want %+v", w.ID, got, w)
		}
	}

	filters := []struct {
		query    string
		wantCode int
		wantRows int
	}{
		{"?job=J-24003", 200, 10},
		{"?job=J-00000", 200, 0},
		{"?from=2026-09-09", 200, 6},
		{"?to=2026-09-02", 200, 6},
		{"?job=J-24001&from=2026-09-03&to=2026-09-05", 200, 3},
		{"?from=2026-13-01", 400, 0},
		{"?to=09/01/2026", 400, 0},
	}
	for _, f := range filters {
		t.Run(f.query, func(t *testing.T) {
			rec := v.do("GET", "/api/reports"+f.query, nil, ck)
			if rec.Code != f.wantCode {
				t.Fatalf("= %d %s", rec.Code, rec.Body)
			}
			if f.wantCode != 200 {
				if got := errCode(t, rec); got != "bad_request" {
					t.Errorf("code = %q", got)
				}
				return
			}
			got := decodeJSON[struct {
				Rows []row `json:"rows"`
			}](t, rec).Rows
			if got == nil || len(got) != f.wantRows {
				t.Errorf("行数 = %d（nil=%v）, want %d", len(got), got == nil, f.wantRows)
			}
		})
	}
}

func TestJobsAndSummary(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	ck := v.login("manager@" + slotA + ".test")

	rec := v.do("GET", "/api/jobs", nil, ck)
	jobs := decodeJSON[struct {
		Jobs []struct{ ID, Name, Address string } `json:"jobs"`
	}](t, rec).Jobs
	if len(jobs) != 3 || jobs[0].ID != "J-24001" || jobs[2].ID != "J-24003" || jobs[1].Address == "" {
		t.Errorf("jobs = %+v", jobs)
	}

	type summaryRow struct {
		JobID      string  `json:"jobId"`
		JobName    string  `json:"jobName"`
		TotalHours float64 `json:"totalHours"`
		LateCount  int     `json:"lateCount"`
	}
	tests := []struct {
		query string
		want  []summaryRow
	}{
		{"", []summaryRow{
			{"J-24001", "滨江示范小学扩建（虚构）", 236, 1},
			{"J-24002", "青石社区卫生站（虚构）", 196, 2},
			{"J-24003", "北岸物流仓库（虚构）", 284, 3},
		}},
		{"?from=2026-09-02&to=2026-09-02", []summaryRow{
			{"J-24001", "滨江示范小学扩建（虚构）", 33.5, 0},
			{"J-24002", "青石社区卫生站（虚构）", 25.5, 1},
			{"J-24003", "北岸物流仓库（虚构）", 41.5, 1},
		}},
		// 09-10 三个工地都未提交：没有任何行，返回空数组。
		{"?from=2026-09-10", []summaryRow{}},
	}
	for _, tt := range tests {
		t.Run("summary"+tt.query, func(t *testing.T) {
			rec := v.do("GET", "/api/summary"+tt.query, nil, ck)
			if rec.Code != 200 {
				t.Fatalf("= %d %s", rec.Code, rec.Body)
			}
			if tt.query == "?from=2026-09-10" && strings.TrimSpace(rec.Body.String()) != `{"rows":[]}` {
				t.Errorf("body = %s", rec.Body)
			}
			got := decodeJSON[struct {
				Rows []summaryRow `json:"rows"`
			}](t, rec).Rows
			if len(got) != len(tt.want) {
				t.Fatalf("rows = %+v", got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("rows[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
	if rec := v.do("GET", "/api/summary?from=x", nil, ck); rec.Code != 400 || errCode(t, rec) != "bad_request" {
		t.Errorf("坏日期 = %d %s", rec.Code, rec.Body)
	}
}

// TestReportFlow 走一遍新建、改工时、提交、备注、锁定，检查每一步的错误 code。
func TestReportFlow(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	// 会话 12 小时有效，登录时间要离下面各步的时钟足够近。
	v.now = time.Date(2026, 9, 21, 20, 0, 0, 0, time.UTC)
	field := v.login("field@" + slotA + ".test")
	manager := v.login("manager@" + slotA + ".test")

	type step struct {
		name       string
		ck         string
		method     string
		path       string
		body       any
		now        time.Time
		wantStatus int
		wantCode   string
		check      func(t *testing.T, d detail)
	}
	cks := map[string]*http.Cookie{"field": field, "manager": manager}
	var id int64
	path := func(suffix string) string { return fmt.Sprintf("/api/reports/%d%s", id, suffix) }
	long := strings.Repeat("长", 501)
	steps := []step{
		{"新建", "field", "POST", "/api/reports", map[string]string{"jobId": "J-24002", "businessDate": "2026-09-20"}, time.Time{}, 201, "", func(t *testing.T, d detail) {
			id = d.ID
			if d.Status != "未提交" || d.SubmittedAt != nil || d.Locked || d.TotalHours != 0 || d.DaysLate != 0 ||
				d.Crew == nil || d.Notes == nil || d.JobName != "青石社区卫生站（虚构）" || d.BusinessDate != "2026-09-20" {
				t.Errorf("detail = %+v", d)
			}
			if d.Can != (can{true, true, false, true}) {
				t.Errorf("can = %+v", d.Can)
			}
		}},
		{"重复新建", "field", "POST", "/api/reports", map[string]string{"jobId": "J-24002", "businessDate": "2026-09-20"}, time.Time{}, 409, "already_exists", nil},
		{"新建：日期格式不对", "field", "POST", "/api/reports", map[string]string{"jobId": "J-24002", "businessDate": "2026/09/20"}, time.Time{}, 400, "bad_request", nil},
		{"新建：缺 jobId", "field", "POST", "/api/reports", map[string]string{"businessDate": "2026-09-21"}, time.Time{}, 400, "bad_request", nil},
		{"新建：工地不存在", "field", "POST", "/api/reports", map[string]string{"jobId": "J-99999", "businessDate": "2026-09-21"}, time.Time{}, 400, "bad_request", nil},
		{"新建：请求体过大", "field", "POST", "/api/reports", `{"jobId":"` + strings.Repeat("x", 70*1024) + `"}`, time.Time{}, 413, "bad_request", nil},
		{"新建：不是 JSON", "field", "POST", "/api/reports", "not json", time.Time{}, 400, "bad_request", nil},
		{"工时为负", "field", "PUT", "", map[string]any{"crew": []map[string]any{{"worker": "张三", "hours": -1}}}, time.Time{}, 422, "negative_hours", nil},
		{"工人姓名为空", "field", "PUT", "", map[string]any{"crew": []map[string]any{{"worker": "  ", "hours": 1}}}, time.Time{}, 400, "bad_request", nil},
		{"工人重复", "field", "PUT", "", map[string]any{"crew": []map[string]any{{"worker": "甲", "hours": 1}, {"worker": " 甲 ", "hours": 2}}}, time.Time{}, 400, "bad_request", nil},
		{"缺 crew", "field", "PUT", "", map[string]any{}, time.Time{}, 400, "bad_request", nil},
		{"改工时", "field", "PUT", "", map[string]any{"crew": []map[string]any{{"worker": "乙", "hours": 4.5}, {"worker": " 甲", "hours": 8}}}, time.Time{}, 200, "", func(t *testing.T, d detail) {
			if len(d.Crew) != 2 || d.Crew[0].Worker != "乙" || d.Crew[1].Worker != "甲" || d.TotalHours != 12.5 {
				t.Errorf("crew = %+v, total = %v", d.Crew, d.TotalHours)
			}
		}},
		{"整体替换工时", "field", "PUT", "", map[string]any{"crew": []map[string]any{{"worker": "丙", "hours": 6}}}, time.Time{}, 200, "", func(t *testing.T, d detail) {
			if len(d.Crew) != 1 || d.Crew[0].Worker != "丙" || d.TotalHours != 6 {
				t.Errorf("crew = %+v", d.Crew)
			}
		}},
		// 业务日期 09-20，09-22 上海时间 08:00 提交：迟交 2 天。
		{"迟交 2 天提交", "field", "POST", "/submit", nil, time.Date(2026, 9, 22, 8, 0, 0, 0, shanghai), 200, "", func(t *testing.T, d detail) {
			if d.Status != "已提交" || d.DaysLate != 2 || d.SubmittedAt == nil || *d.SubmittedAt != "2026-09-22T00:00:00Z" {
				t.Errorf("detail = %+v submittedAt=%v", d, d.SubmittedAt)
			}
			if d.Can != (can{true, false, false, true}) {
				t.Errorf("can = %+v", d.Can)
			}
		}},
		{"重复提交", "field", "POST", "/submit", nil, time.Time{}, 409, "already_submitted", nil},
		{"空备注", "field", "POST", "/notes", map[string]string{"body": "   "}, time.Time{}, 422, "empty_note", nil},
		{"备注过长", "field", "POST", "/notes", map[string]string{"body": long}, time.Time{}, 422, "note_too_long", nil},
		{"备注 500 字", "field", "POST", "/notes", map[string]string{"body": long[:len(long)-len("长")]}, time.Time{}, 201, "", nil},
		{"第二条备注", "manager", "POST", "/notes", map[string]string{"body": "请补照片"}, time.Date(2026, 9, 22, 3, 0, 0, 0, time.UTC), 201, "", func(t *testing.T, d detail) {
			if len(d.Notes) != 2 || d.Notes[0].Body != "请补照片" || d.Notes[0].AuthorName != "测试项目经理" ||
				d.Notes[0].CreatedAt != "2026-09-22T03:00:00Z" || d.Notes[1].AuthorName != "测试现场" || d.Notes[0].ID <= d.Notes[1].ID {
				t.Errorf("notes = %+v", d.Notes)
			}
		}},
		{"锁定", "manager", "POST", "/lock", nil, time.Time{}, 200, "", func(t *testing.T, d detail) {
			if !d.Locked || d.Status != "已锁定" || d.DaysLate != 2 || d.Can != (can{false, false, false, true}) {
				t.Errorf("detail = %+v", d)
			}
		}},
		{"重复锁定", "manager", "POST", "/lock", nil, time.Time{}, 409, "locked", nil},
		{"锁定后改工时", "field", "PUT", "", map[string]any{"crew": []map[string]any{{"worker": "丙", "hours": 1}}}, time.Time{}, 409, "locked", nil},
		{"锁定后提交", "field", "POST", "/submit", nil, time.Time{}, 409, "locked", nil},
		{"锁定后仍可追加备注", "field", "POST", "/notes", map[string]string{"body": "锁定后的补充"}, time.Time{}, 201, "", func(t *testing.T, d detail) {
			if len(d.Notes) != 3 || d.Notes[0].Body != "锁定后的补充" || len(d.Crew) != 1 || d.Crew[0].Hours != 6 {
				t.Errorf("detail = %+v", d)
			}
		}},
		{"不存在的日报", "field", "GET", "/api/reports/99999", nil, time.Time{}, 404, "not_found", nil},
		{"id 不是数字", "field", "GET", "/api/reports/abc", nil, time.Time{}, 404, "not_found", nil},
		{"改不存在的日报", "field", "POST", "/api/reports/99999/submit", nil, time.Time{}, 404, "not_found", nil},
	}
	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			if !s.now.IsZero() {
				v.now = s.now
			}
			p := s.path
			switch {
			case s.method == "PUT":
				p = path("/crew")
			case strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "/api/"):
				p = path(p)
			}
			rec := v.do(s.method, p, s.body, cks[s.ck])
			if rec.Code != s.wantStatus {
				t.Fatalf("%s %s = %d, want %d：%s", s.method, p, rec.Code, s.wantStatus, rec.Body)
			}
			if s.wantCode != "" {
				if got := errCode(t, rec); got != s.wantCode {
					t.Errorf("code = %q, want %q", got, s.wantCode)
				}
			}
			if s.check != nil {
				s.check(t, decodeJSON[detail](t, rec))
			}
		})
	}

	t.Run("负工时的提示", func(t *testing.T) {
		rec := v.do("POST", "/api/reports", map[string]string{"jobId": "J-24003", "businessDate": "2026-09-20"}, field)
		d := decodeJSON[detail](t, rec)
		rec = v.do("PUT", fmt.Sprintf("/api/reports/%d/crew", d.ID), map[string]any{"crew": []map[string]any{{"worker": "张三", "hours": -1}}}, field)
		if !strings.Contains(rec.Body.String(), `"message":"工时不能为负数：张三=-1"`) {
			t.Errorf("body = %s", rec.Body)
		}
	})
}

// TestTenantIsolation：一个租户的用户读不到、改不了另一个租户的日报。
func TestTenantIsolation(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	testdb.Seed(t, slotB)
	v := newEnv(t, pool)
	a := v.login("admin@" + slotA + ".test")
	b := v.login("admin@" + slotB + ".test")

	rec := v.do("POST", "/api/reports", map[string]string{"jobId": "J-24001", "businessDate": "2026-09-25"}, b)
	if rec.Code != 201 {
		t.Fatalf("B 新建 = %d %s", rec.Code, rec.Body)
	}
	id := decodeJSON[detail](t, rec).ID

	tests := []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/reports/%d", nil},
		{"PUT", "/api/reports/%d/crew", map[string]any{"crew": []map[string]any{{"worker": "甲", "hours": 1}}}},
		{"POST", "/api/reports/%d/submit", nil},
		{"POST", "/api/reports/%d/lock", nil},
		{"POST", "/api/reports/%d/notes", map[string]string{"body": "越权"}},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := v.do(tt.method, fmt.Sprintf(tt.path, id), tt.body, a)
			if rec.Code != 404 || errCode(t, rec) != "not_found" {
				t.Errorf("= %d %s", rec.Code, rec.Body)
			}
		})
	}

	rec = v.do("GET", "/api/reports?from=2026-09-25", nil, a)
	if strings.TrimSpace(rec.Body.String()) != `{"rows":[]}` {
		t.Errorf("A 的台账里出现了 B 的日报：%s", rec.Body)
	}
	rec = v.do("GET", fmt.Sprintf("/api/reports/%d", id), nil, b)
	if d := decodeJSON[detail](t, rec); rec.Code != 200 || d.Status != "未提交" {
		t.Errorf("A 的越权请求改了 B 的日报：%d %s", rec.Code, rec.Body)
	}
	rec = v.do("GET", "/api/me", nil, b)
	if me := decodeJSON[map[string]any](t, rec); me["tenant"] != slotB {
		t.Errorf("me = %v", me)
	}
}

// TestTenantOutdated：租户 schema 的迁移版本落后时返回 503 tenant_outdated，补上后恢复。
func TestTenantOutdated(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotC)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `update `+slotC+`.goose_db_version set is_applied = false where version_id = 1`); err != nil {
		t.Fatal(err)
	}
	outdated, err := httpapi.CheckTenants(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range outdated {
		found = found || n == slotC
	}
	if !found {
		t.Errorf("CheckTenants = %v，没有 %s", outdated, slotC)
	}

	v := newEnv(t, pool)
	ck := v.login("finance@" + slotC + ".test")
	if rec := v.do("GET", "/api/me", nil, ck); rec.Code != 200 {
		t.Errorf("/api/me 不依赖租户 schema，= %d", rec.Code)
	}
	for _, p := range []string{"/api/jobs", "/api/reports", "/api/reports/1", "/api/summary"} {
		rec := v.do("GET", p, nil, ck)
		if rec.Code != 503 || errCode(t, rec) != "tenant_outdated" {
			t.Errorf("%s = %d %s", p, rec.Code, rec.Body)
		}
	}
	if _, err := pool.Exec(ctx, `update `+slotC+`.goose_db_version set is_applied = true where version_id = 1`); err != nil {
		t.Fatal(err)
	}
	if rec := v.do("GET", "/api/reports", nil, ck); rec.Code != 200 {
		t.Errorf("补上迁移后 = %d %s", rec.Code, rec.Body)
	}
}

func TestOrigin(t *testing.T) {
	pool := testdb.Pool(t)
	testdb.Seed(t, slotA)
	v := newEnv(t, pool)
	ck := v.login("field@" + slotA + ".test")
	note := map[string]string{"body": "来源检查"}
	// httptest 请求的 Host 是 example.com。
	tests := []struct {
		name   string
		method string
		path   string
		origin string
		want   int
	}{
		{"没有 Origin", "POST", "/api/reports/6/notes", "", 201},
		{"同源", "POST", "/api/reports/6/notes", "http://example.com", 201},
		{"同源 https", "POST", "/api/reports/6/notes", "https://example.com", 201},
		{"跨源", "POST", "/api/reports/6/notes", "http://evil.example", 403},
		{"端口不同", "POST", "/api/reports/6/notes", "http://example.com:8443", 403},
		{"Origin 为 null", "POST", "/api/reports/6/notes", "null", 403},
		{"跨源登录", "POST", "/api/login", "http://evil.example", 403},
		{"跨源 PUT", "PUT", "/api/reports/9/crew", "http://evil.example", 403},
		{"跨源 GET 不受限", "GET", "/api/reports/6", "http://evil.example", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hdr []string
			if tt.origin != "" {
				hdr = []string{"Origin", tt.origin}
			}
			var body any
			if tt.method != "GET" {
				body = note
			}
			rec := v.do(tt.method, tt.path, body, ck, hdr...)
			if rec.Code != tt.want {
				t.Fatalf("= %d, want %d：%s", rec.Code, tt.want, rec.Body)
			}
			if tt.want == 403 {
				if got := errCode(t, rec); got != "bad_origin" {
					t.Errorf("code = %q, want bad_origin", got)
				}
			}
		})
	}
}

func TestStaticSPA(t *testing.T) {
	pool := testdb.Pool(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>index</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := newEnv(t, pool, func(o *httpapi.Options) { o.WebDir = dir })
	tests := []struct {
		path     string
		wantCode int
		contains string
	}{
		{"/", 200, "<title>index</title>"},
		{"/reports/12", 200, "<title>index</title>"},
		{"/login", 200, "<title>index</title>"},
		{"/assets/app.js", 200, "console.log"},
		{"/api/unknown", 404, `"not_found"`},
		{"/api/reports", 401, `"unauthenticated"`},
		{"/healthz", 200, `"ok"`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := v.do("GET", tt.path, nil, nil)
			if rec.Code != tt.wantCode || !strings.Contains(rec.Body.String(), tt.contains) {
				t.Errorf("= %d %s", rec.Code, rec.Body)
			}
		})
	}
}
