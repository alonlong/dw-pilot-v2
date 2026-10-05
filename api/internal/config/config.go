// Package config 从环境变量读取服务配置。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
	// 运行镜像里可能没有时区数据库，内嵌一份。
	_ "time/tzdata"
)

// DefaultBusinessTZ 是业务日期所在的时区：迟交天数按这个时区的自然日计算。
const DefaultBusinessTZ = "Asia/Shanghai"

// Config 是服务的配置。
type Config struct {
	// DatabaseURL 是 Postgres 连接串（DATABASE_URL，必填）。
	DatabaseURL string
	// Addr 是监听地址（ADDR，默认 ":8080"）。
	Addr string
	// WebDir 是前端构建产物目录（WEB_DIR，可选）；为空时不提供静态文件。
	WebDir string
	// BuildSHA 是构建版本（BUILD_SHA，默认 "dev"）。
	BuildSHA string
	// CookieSecure 为真时会话 Cookie 带 Secure（COOKIE_SECURE=1 或 true）。
	CookieSecure bool
	// BusinessLocation 是业务时区（BUSINESS_TZ，默认 Asia/Shanghai）。
	BusinessLocation *time.Location
}

// Load 读取环境变量；DATABASE_URL 为空或 BUSINESS_TZ 无效时返回错误。
func Load() (Config, error) {
	c := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		Addr:         envOr("ADDR", ":8080"),
		WebDir:       os.Getenv("WEB_DIR"),
		BuildSHA:     envOr("BUILD_SHA", "dev"),
		CookieSecure: truthy(os.Getenv("COOKIE_SECURE")),
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL 未设置")
	}
	loc, err := LoadLocation()
	if err != nil {
		return Config{}, err
	}
	c.BusinessLocation = loc
	return c, nil
}

// LoadLocation 读取 BUSINESS_TZ，未设置时用 DefaultBusinessTZ。
func LoadLocation() (*time.Location, error) {
	name := envOr("BUSINESS_TZ", DefaultBusinessTZ)
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("BUSINESS_TZ=%q 无效：%w", name, err)
	}
	return loc, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
