package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"example.com/dailylog/api/internal/httpapi"
	"example.com/dailylog/api/internal/testdb"
)

// 每个测试用到的槽位；与其他测试包的槽位不同。
const (
	slotA = "dlpv981"
	slotB = "dlpv982"
	slotC = "dlpv983"
)

var shanghai = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}()

// env 是一个测试服务：可替换的时钟、连到测试库的连接池。
type env struct {
	t    *testing.T
	pool *pgxpool.Pool
	e    *echo.Echo
	now  time.Time
}

type option func(*httpapi.Options)

func newEnv(t *testing.T, pool *pgxpool.Pool, opts ...option) *env {
	t.Helper()
	v := &env{t: t, pool: pool, now: time.Date(2026, 9, 11, 2, 0, 0, 0, time.UTC)}
	o := httpapi.Options{
		Pool:     pool,
		Now:      func() time.Time { return v.now },
		Location: shanghai,
		BuildSHA: "test-sha",
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, fn := range opts {
		fn(&o)
	}
	e, err := httpapi.New(o)
	if err != nil {
		t.Fatal(err)
	}
	v.e = e
	return v
}

// do 发一个请求；body 为 string 时原样发送，否则编码为 JSON。hdr 是成对的头名和值。
func (v *env) do(method, path string, body any, ck *http.Cookie, hdr ...string) *httptest.ResponseRecorder {
	v.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			v.t.Fatal(err)
		}
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if ck != nil {
		req.AddCookie(ck)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	rec := httptest.NewRecorder()
	v.e.ServeHTTP(rec, req)
	return rec
}

// login 用测试账号登录，返回会话 Cookie。
func (v *env) login(email string) *http.Cookie {
	v.t.Helper()
	rec := v.do(http.MethodPost, "/api/login", map[string]string{"email": email, "password": testdb.Password}, nil)
	if rec.Code != http.StatusOK {
		v.t.Fatalf("登录 %s：%d %s", email, rec.Code, rec.Body)
	}
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == httpapi.CookieName {
			return ck
		}
	}
	v.t.Fatalf("登录 %s 没有设置 Cookie", email)
	return nil
}

// loginAll 登录槽位的四个默认账号，按角色返回 Cookie。
func (v *env) loginAll(slot string) map[string]*http.Cookie {
	out := map[string]*http.Cookie{}
	for _, r := range []string{"admin", "manager", "field", "finance"} {
		out[r] = v.login(r + "@" + slot + ".test")
	}
	return out
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("错误响应不是 JSON：%s", rec.Body)
	}
	if body.Error.Message == "" {
		t.Errorf("错误响应没有 message：%s", rec.Body)
	}
	return body.Error.Code
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("响应不是预期的 JSON：%v\n%s", err, rec.Body)
	}
	return v
}

type can struct {
	EditCrew bool `json:"editCrew"`
	Submit   bool `json:"submit"`
	Lock     bool `json:"lock"`
	AddNote  bool `json:"addNote"`
}

type detail struct {
	ID           int64   `json:"id"`
	JobID        string  `json:"jobId"`
	JobName      string  `json:"jobName"`
	BusinessDate string  `json:"businessDate"`
	Status       string  `json:"status"`
	SubmittedAt  *string `json:"submittedAt"`
	Locked       bool    `json:"locked"`
	TotalHours   float64 `json:"totalHours"`
	DaysLate     int     `json:"daysLate"`
	Crew         []struct {
		Worker string  `json:"worker"`
		Hours  float64 `json:"hours"`
	} `json:"crew"`
	Notes []struct {
		ID         int64  `json:"id"`
		Body       string `json:"body"`
		AuthorName string `json:"authorName"`
		CreatedAt  string `json:"createdAt"`
	} `json:"notes"`
	Can can `json:"can"`
}

type row struct {
	ID           int64   `json:"id"`
	JobID        string  `json:"jobId"`
	JobName      string  `json:"jobName"`
	BusinessDate string  `json:"businessDate"`
	Status       string  `json:"status"`
	TotalHours   float64 `json:"totalHours"`
	DaysLate     int     `json:"daysLate"`
}
