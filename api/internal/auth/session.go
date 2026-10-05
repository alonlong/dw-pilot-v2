package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"example.com/dailylog/api/internal/db"
)

// SessionTTL 是会话有效期。
const SessionTTL = 12 * time.Hour

// 登录与会话的错误。
var (
	// ErrBadCredentials 表示邮箱或密码错误（不区分是哪一个）。
	ErrBadCredentials = errors.New("auth: bad credentials")
	// ErrNoSession 表示没有会话、会话已过期或用户已停用。
	ErrNoSession = errors.New("auth: no session")
)

// User 是已登录的用户。
type User struct {
	ID         int64
	Tenant     string
	TenantName string
	Email      string
	Name       string
	Role       Role
}

const userColumns = `u.id, u.tenant, t.display_name, u.email, u.display_name, u.role`

func scanUser(row pgx.Row, extra ...any) (*User, error) {
	var u User
	var role string
	dest := append([]any{&u.ID, &u.Tenant, &u.TenantName, &u.Email, &u.Name, &role}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	u.Role = Role(role)
	return &u, nil
}

// NormalizeEmail 去掉首尾空格并转为小写。
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Login 按邮箱和密码查找用户；租户由账号决定。邮箱不存在、密码错误、用户停用都返回
// ErrBadCredentials。同一邮箱在多个租户下且密码都匹配时无法确定租户，也拒绝登录。
func Login(ctx context.Context, q db.Querier, email, password string) (*User, error) {
	email = NormalizeEmail(email)
	if email == "" || password == "" {
		return nil, ErrBadCredentials
	}
	rows, err := q.Query(ctx, `select `+userColumns+`, u.password_hash
		from public.app_user u join public.tenant t on t.name = u.tenant
		where u.email = $1 and not u.disabled
		order by u.tenant`, email)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		user *User
		hash string
	}
	var cands []candidate
	for rows.Next() {
		var hash string
		u, err := scanUser(rows, &hash)
		if err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, candidate{u, hash})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cands) == 0 {
		_, _ = VerifyPassword(dummyHash, password)
		return nil, ErrBadCredentials
	}
	var matched []*User
	for _, c := range cands {
		ok, err := VerifyPassword(c.hash, password)
		if err != nil {
			slog.Error("用户的密码哈希格式不对", "user", c.user.ID, "err", err)
			continue
		}
		if ok {
			matched = append(matched, c.user)
		}
	}
	if len(matched) != 1 {
		if len(matched) > 1 {
			slog.Warn("同一邮箱在多个租户下密码相同，拒绝登录", "email", email)
		}
		return nil, ErrBadCredentials
	}
	return matched[0], nil
}

// NewSession 为用户新建会话，返回放进 Cookie 的令牌和过期时间；同时清理该用户已过期的会话。
func NewSession(ctx context.Context, q db.Querier, userID int64, now time.Time) (token string, expires time.Time, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	expires = now.Add(SessionTTL)
	if _, err := q.Exec(ctx, `delete from public.session where user_id = $1 and expires_at <= $2`, userID, now); err != nil {
		return "", time.Time{}, err
	}
	if _, err := q.Exec(ctx, `insert into public.session (id, user_id, expires_at, created_at) values ($1, $2, $3, $4)`,
		hashToken(token), userID, expires, now); err != nil {
		return "", time.Time{}, fmt.Errorf("新建会话：%w", err)
	}
	return token, expires, nil
}

// LookupSession 返回令牌对应的用户；令牌无效、已过期或用户已停用时返回 ErrNoSession。
func LookupSession(ctx context.Context, q db.Querier, token string, now time.Time) (*User, error) {
	if token == "" {
		return nil, ErrNoSession
	}
	u, err := scanUser(q.QueryRow(ctx, `select `+userColumns+`
		from public.session s
		join public.app_user u on u.id = s.user_id
		join public.tenant t on t.name = u.tenant
		where s.id = $1 and s.expires_at > $2 and not u.disabled`, hashToken(token), now))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoSession
	}
	return u, err
}

// DeleteSession 删除令牌对应的会话；不存在时什么也不做。
func DeleteSession(ctx context.Context, q db.Querier, token string) error {
	if token == "" {
		return nil
	}
	_, err := q.Exec(ctx, `delete from public.session where id = $1`, hashToken(token))
	return err
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
