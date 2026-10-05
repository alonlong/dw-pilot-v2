// Package httpapi 实现 /api 下的路由与处理函数，接口契约见 docs/api.md。
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"example.com/dailylog/api/internal/auth"
	"example.com/dailylog/api/internal/db"
	"example.com/dailylog/api/internal/store"
)

// CookieName 是会话 Cookie 的名字。
const CookieName = "dl_session"

// Options 是服务的依赖与设置。
type Options struct {
	Pool *pgxpool.Pool
	// Now 返回当前时间；测试可替换。为空时用 time.Now。
	Now func() time.Time
	// Location 是业务时区。
	Location *time.Location
	// BuildSHA 是 /api/version 返回的版本。
	BuildSHA string
	// CookieSecure 为真时会话 Cookie 带 Secure。
	CookieSecure bool
	// WebDir 是前端构建产物目录；为空时不提供静态文件。
	WebDir string
	// Log 为空时用 slog.Default()。
	Log *slog.Logger
}

type server struct {
	Options
	tenantTarget int64
	// upToDate 缓存迁移版本已是最新的租户；落后的租户每次请求都重新检查，
	// 运维迁移后不用重启服务就能恢复。
	upToDate sync.Map
}

// New 返回配置好路由的 Echo 实例。
func New(opts Options) (*echo.Echo, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	if opts.Location == nil {
		opts.Location = time.UTC
	}
	if opts.BuildSHA == "" {
		opts.BuildSHA = "dev"
	}
	target, err := db.TenantTarget()
	if err != nil {
		return nil, err
	}
	s := &server{Options: opts, tenantTarget: target}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = s.handleError
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("64K"))
	if opts.WebDir != "" {
		e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
			Root:  opts.WebDir,
			HTML5: true,
			Skipper: func(c echo.Context) bool {
				p := c.Request().URL.Path
				m := c.Request().Method
				return (m != http.MethodGet && m != http.MethodHead) ||
					p == "/api" || strings.HasPrefix(p, "/api/") || p == "/healthz"
			},
		}))
	}

	e.Use(s.checkOrigin)

	// 中间件按路由挂，不用 Group 的中间件：Echo 会为带中间件的 Group 注册兜底路由，
	// 未知的 /api 路径就会先被要求登录，而不是返回 404。
	sess := s.requireSession
	tenant := func(a auth.Action) []echo.MiddlewareFunc {
		return []echo.MiddlewareFunc{s.requireSession, need(a), s.requireTenantCurrent}
	}
	e.GET("/healthz", s.healthz)
	api := e.Group("/api")
	api.GET("/version", s.version)
	api.POST("/login", s.login)
	api.POST("/logout", s.logout)
	api.GET("/me", s.me, sess)
	api.GET("/jobs", s.listJobs, tenant(auth.View)...)
	api.GET("/reports", s.listReports, tenant(auth.View)...)
	api.POST("/reports", s.createReport, tenant(auth.EditReport)...)
	api.GET("/reports/:id", s.getReport, tenant(auth.View)...)
	api.PUT("/reports/:id/crew", s.saveCrew, tenant(auth.EditReport)...)
	api.POST("/reports/:id/submit", s.submit, tenant(auth.EditReport)...)
	api.POST("/reports/:id/lock", s.lock, tenant(auth.Lock)...)
	api.POST("/reports/:id/notes", s.addNote, tenant(auth.AddNote)...)
	api.GET("/summary", s.summary, tenant(auth.ViewSummary)...)
	return e, nil
}

// CheckTenants 检查每个已登记租户的迁移版本，落后或缺 schema 的记日志；返回落后的租户。
// 服务启动时调用；之后请求到来时还会按租户再检查（见 requireTenantCurrent）。
func CheckTenants(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) ([]string, error) {
	want, err := db.TenantTarget()
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `select name from public.tenant order by name`)
	if err != nil {
		return nil, err
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	var outdated []string
	for _, n := range names {
		v, err := db.TenantVersion(ctx, pool, n)
		if err != nil || v < want {
			log.Error("租户的迁移版本落后，拒绝该租户的请求", "tenant", n, "version", v, "want", want, "err", err)
			outdated = append(outdated, n)
		}
	}
	return outdated, nil
}

// ---- 错误 ----

// apiError 是返回给客户端的错误。
type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

func newError(status int, code, msg string) *apiError {
	return &apiError{Status: status, Code: code, Message: msg}
}

var (
	errUnauthenticated = newError(http.StatusUnauthorized, "unauthenticated", "请先登录")
	errBadCredentials  = newError(http.StatusUnauthorized, "bad_credentials", "邮箱或密码错误")
	errForbidden       = newError(http.StatusForbidden, "forbidden", "当前角色不能执行此操作")
	errBadOrigin       = newError(http.StatusForbidden, "bad_origin", "跨源请求被拒绝")
	errNotFound        = newError(http.StatusNotFound, "not_found", "资源不存在")
	errTenantOutdated  = newError(http.StatusServiceUnavailable, "tenant_outdated", "租户数据版本落后，请联系管理员")
	errLocked          = newError(http.StatusConflict, "locked", "日报已锁定，不能改工时或提交")
)

func badRequest(msg string) *apiError {
	return newError(http.StatusBadRequest, "bad_request", msg)
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *server) handleError(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	var ae *apiError
	var he *echo.HTTPError
	switch {
	case errors.As(err, &ae):
	case errors.As(err, &he):
		switch he.Code {
		case http.StatusNotFound:
			ae = errNotFound
		case http.StatusRequestEntityTooLarge:
			ae = newError(he.Code, "bad_request", "请求体过大")
		case http.StatusMethodNotAllowed:
			ae = newError(he.Code, "bad_request", "不支持的请求方法")
		default:
			ae = newError(he.Code, "bad_request", http.StatusText(he.Code))
		}
	default:
		s.Log.Error("请求处理失败", "method", c.Request().Method, "path", c.Request().URL.Path, "err", err)
		ae = newError(http.StatusInternalServerError, "internal", "服务器内部错误")
	}
	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(ae.Status)
		return
	}
	_ = c.JSON(ae.Status, errorBody{Error: errorDetail{Code: ae.Code, Message: ae.Message}})
}

// ---- 中间件 ----

// checkOrigin 拒绝带 Origin 头且与 Host 不同源的写请求。
func (s *server) checkOrigin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		switch req.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return next(c)
		}
		origin := req.Header.Get("Origin")
		if origin == "" {
			return next(c)
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, req.Host) {
			return errBadOrigin
		}
		return next(c)
	}
}

const userKey = "user"

func currentUser(c echo.Context) *auth.User {
	u, _ := c.Get(userKey).(*auth.User)
	return u
}

// requireSession 读取会话 Cookie，没有有效会话时返回 401。
func (s *server) requireSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ck, err := c.Cookie(CookieName)
		if err != nil {
			return errUnauthenticated
		}
		u, err := auth.LookupSession(c.Request().Context(), s.Pool, ck.Value, s.Now())
		if errors.Is(err, auth.ErrNoSession) {
			s.clearCookie(c)
			return errUnauthenticated
		}
		if err != nil {
			return err
		}
		c.Set(userKey, u)
		return next(c)
	}
}

// requireTenantCurrent 在当前用户租户的迁移版本落后时返回 503 tenant_outdated。
func (s *server) requireTenantCurrent(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		tenant := currentUser(c).Tenant
		if _, ok := s.upToDate.Load(tenant); ok {
			return next(c)
		}
		v, err := db.TenantVersion(c.Request().Context(), s.Pool, tenant)
		if err != nil && !errors.Is(err, db.ErrNoTenantSchema) {
			return err
		}
		if err != nil || v < s.tenantTarget {
			s.Log.Error("租户的迁移版本落后，拒绝请求", "tenant", tenant, "version", v, "want", s.tenantTarget, "err", err)
			return errTenantOutdated
		}
		s.upToDate.Store(tenant, true)
		return next(c)
	}
}

// need 要求当前用户的角色允许操作 a。
func need(a auth.Action) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !currentUser(c).Role.Can(a) {
				return errForbidden
			}
			return next(c)
		}
	}
}

// inTenant 在当前用户租户的事务里执行 fn。
func (s *server) inTenant(c echo.Context, fn func(*store.Store) error) error {
	ctx := c.Request().Context()
	return db.InTenant(ctx, s.Pool, currentUser(c).Tenant, func(tx pgx.Tx) error {
		return fn(store.New(tx, s.Location))
	})
}

func (s *server) setCookie(c echo.Context, token string, expires time.Time) {
	c.SetCookie(&http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(auth.SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *server) clearCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
