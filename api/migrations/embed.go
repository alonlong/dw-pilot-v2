// Package migrations 内嵌 public 与租户 schema 的 Goose SQL 迁移。
//
// public/ 只在 public schema 上执行一次；tenant/ 在每个租户 schema 上各执行一次，
// 执行时 search_path 指向该租户，所以 tenant/ 里的表名不带 schema，引用 public 的表要写全名。
package migrations

import "embed"

// FS 包含 public/*.sql 与 tenant/*.sql。
//
//go:embed public/*.sql tenant/*.sql
var FS embed.FS
