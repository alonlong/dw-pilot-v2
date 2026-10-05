# dw-pilot-v2

施工日报试点项目：Go API + Postgres（schema-per-tenant）+ React SPA。设计见 [`docs/design.md`](docs/design.md)，接口契约见 [`docs/api.md`](docs/api.md)。

业务规则只在 `daily/`（纯函数，时间由调用方传入）；`api/` 负责存储、登录与权限、HTTP 接口和租户命令。

## 目录

```
daily/                  领域包：迟交天数、工时合计、状态、备注校验
api/cmd/server/         HTTP 服务：/api/* + 静态前端
api/cmd/tenant/         预览槽位租户：check / seed / reset / drop
api/cmd/devseed/        本地开发租户 dev（仅本地使用）
api/internal/config/    环境变量
api/internal/db/        连接池、按租户切 search_path、迁移
api/internal/auth/      密码（argon2id）、会话、角色与权限
api/internal/store/     日报读写（SQL ↔ daily.Report）
api/internal/httpapi/   路由与处理函数
api/internal/tenant/    快照读取与校验、租户建删
api/migrations/         Goose SQL 迁移（public/、tenant/，内嵌进二进制）
api/testdata/golden/    虚构的 Golden 快照
```

## 本地运行

需要 Go 1.25+ 和 Postgres 16。

1. 起一个 Postgres（任选其一）：

   ```sh
   docker run -d --name dlpilot-pg -p 5432:5432 \
     -e POSTGRES_USER=dl -e POSTGRES_PASSWORD=dl -e POSTGRES_DB=dailylog postgres:16
   export DATABASE_URL='postgres://dl:dl@localhost:5432/dailylog?sslmode=disable'
   ```

   或者用已有的空库，把 `DATABASE_URL` 指过去。

2. 建立本地开发租户 `dev`（Golden 快照 + 四个测试账号）：

   ```sh
   export TENANT_TEST_PASSWORD=devpass
   go run ./api/cmd/devseed            # dev 不存在时建立；已存在则报错
   go run ./api/cmd/devseed -reset     # 删除并重建 dev
   ```

   账号：`admin@dev.test`、`manager@dev.test`、`field@dev.test`、`finance@dev.test`，密码都是 `TENANT_TEST_PASSWORD`。

3. 启动服务：

   ```sh
   go run ./api/cmd/server                       # 只提供 /api
   WEB_DIR=web/dist go run ./api/cmd/server      # 同时提供前端构建产物
   ```

4. 试一下：

   ```sh
   curl -s localhost:8080/healthz
   curl -s -c /tmp/dl.cookie -H 'Content-Type: application/json' \
     -d '{"email":"field@dev.test","password":"devpass"}' localhost:8080/api/login
   curl -s -b /tmp/dl.cookie localhost:8080/api/reports
   ```

## 环境变量

| 变量 | 用于 | 说明 |
|---|---|---|
| `DATABASE_URL` | server、tenant、devseed | Postgres 连接串，必填 |
| `ADDR` | server | 监听地址，默认 `:8080` |
| `WEB_DIR` | server | 前端构建产物目录；设置后未知的非 `/api` 路径回退到 `index.html` |
| `BUILD_SHA` | server | `/api/version` 返回的版本，默认 `dev` |
| `COOKIE_SECURE` | server | `1` 或 `true` 时会话 Cookie 带 `Secure` |
| `BUSINESS_TZ` | server | 业务时区，默认 `Asia/Shanghai`；迟交天数按这个时区的自然日计算 |
| `TENANT_TEST_PASSWORD` | tenant、devseed | 测试账号的密码；没有时 seed/reset 拒绝执行 |
| `TENANT_SNAPSHOT` | tenant | `-snapshot` 的默认值 |
| `TEST_DATABASE_URL` | go test | 需要数据库的测试用；没有时跳过这些测试 |

服务启动时跑 public schema 的迁移，并检查每个已登记租户的迁移版本：落后的租户记日志，其请求返回 `503 tenant_outdated`。租户迁移只由 `tenant seed/reset`（或 `devseed`）执行。

## 租户命令

```sh
go run ./api/cmd/tenant check -snapshot api/testdata/golden
go run ./api/cmd/tenant seed  -tenant dlpv1 -snapshot api/testdata/golden [-accounts accounts.json]
go run ./api/cmd/tenant reset -tenant dlpv1 -snapshot api/testdata/golden [-accounts accounts.json]
go run ./api/cmd/tenant drop  -tenant dlpv1
```

- 只接受预览槽位：`dlpv` 加 1～3 位数字，其他名字一律拒绝，没有开关。
- `check` 只读快照、校验行数和 SHA-256，不连接数据库。
- `reset` = 删除（如存在）→ 建 schema 并跑租户迁移 → 导入快照（导入后回读行数）→ 写测试账号；重复执行结果一致。
- 账号文件：`{"tenant": "dlpv1", "accounts": [{"email": "…", "name": "…", "role": "admin|manager|field|finance"}]}`。不给 `-accounts` 时生成 `admin@dlpv1.test`、`manager@dlpv1.test`、`field@dlpv1.test`、`finance@dlpv1.test`。

`devseed` 只处理名为 `dev` 的租户（代码里的常量，不接受租户名参数），不能用来删除别的 schema。

## Golden 快照

`api/testdata/golden`：全部虚构，3 个工地、2026-09-01 至 09-10 共 30 份日报，覆盖未提交、按时提交、迟交 1～3 天、已锁定、有备注、无工时。格式与 abby-pm 相同：`manifest.json` + `tables/<table>.jsonl`。

改了 `tables/` 下的文件后更新 manifest 的行数和 SHA-256：

```sh
go test ./api/internal/tenant -run TestGoldenManifest -update
```

改了租户迁移后，把 manifest 的 `tenant_migration` 改成新的版本号，并按需调整数据。

## 检查

```sh
gofmt -l .
go vet ./...
TEST_DATABASE_URL='postgres://…/空库?sslmode=disable' go test ./...
```

需要数据库的测试会在 `TEST_DATABASE_URL` 指向的库里建删 `dlpv9xx` 槽位，请用专门的空库。
