// Command server 提供 /api/* 接口和（WEB_DIR 设置时）静态前端。
//
// 启动时跑 public schema 的迁移，并检查每个已登记租户的迁移版本：落后的租户记日志，
// 其请求返回 503 tenant_outdated。租户迁移只由 tenant seed/reset 执行，服务不自动迁移
// （避免多个副本同时迁移）。配置见 internal/config。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/dailylog/api/internal/config"
	"example.com/dailylog/api/internal/db"
	"example.com/dailylog/api/internal/httpapi"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	if err := run(log); err != nil {
		log.Error("服务退出", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.MigratePublic(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := httpapi.CheckTenants(ctx, pool, log); err != nil {
		return err
	}

	e, err := httpapi.New(httpapi.Options{
		Pool:         pool,
		Location:     cfg.BusinessLocation,
		BuildSHA:     cfg.BuildSHA,
		CookieSecure: cfg.CookieSecure,
		WebDir:       cfg.WebDir,
		Log:          log,
	})
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("服务启动", "addr", cfg.Addr, "sha", cfg.BuildSHA, "web_dir", cfg.WebDir)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
