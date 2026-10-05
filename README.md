# dw-pilot-v2

施工日报试点项目：Go API + Postgres（schema-per-tenant）+ React SPA。设计见 [`docs/design.md`](docs/design.md)，接口契约见 [`docs/api.md`](docs/api.md)。

业务规则只在 `daily/`（纯函数，时间由调用方传入）；`api/` 负责存储、登录与权限、HTTP 接口和租户命令。给编码 Agent 的规则见 [`AGENTS.md`](AGENTS.md)。

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
web/                    React SPA 与设计系统（web/src/ui/）
e2e/                    Playwright 冒烟测试
Dockerfile              镜像：server、tenant、devseed + 前端 + Golden 快照
docker-compose.yml      本地：postgres + app
```

## 用 docker compose 运行

```sh
docker compose up --build
```

打开 <http://localhost:8080>，用下面任一账号登录，密码 `pilot-pass`（`TENANT_TEST_PASSWORD` 的默认值，只用于本地）：

| 账号 | 角色 |
|---|---|
| `admin@dev.test` | 管理员 |
| `manager@dev.test` | 项目经理 |
| `field@dev.test` | 现场 |
| `finance@dev.test` | 财务 |

- 每次 `up` 时一次性服务 `init` 执行 `devseed -reset`，按 Golden 快照重建 `dev` 租户，之前的改动会丢掉。
- 8080 被占用时换端口：`APP_PORT=18080 docker compose up --build`。
- 换密码：`TENANT_TEST_PASSWORD=… docker compose up --build`。
- 在同一个库里建预览槽位（与平台的重置方式相同）：

  ```sh
  docker compose run --rm --no-deps -e TENANT_TEST_PASSWORD=pilot-pass app tenant reset -tenant dlpv1
  ```

## 镜像

```sh
docker build --build-arg BUILD_SHA=$(git rev-parse HEAD) -t dw-pilot-v2 .
```

运行镜像是 distroless（非 root，无 shell），`server`、`tenant`、`devseed` 都在 PATH 里，只设 `CMD ["server"]`，所以 `docker run <镜像> tenant reset -tenant dlpv1` 直接调用租户命令。镜像里 `WEB_DIR=/app/web`、`TENANT_SNAPSHOT=/app/golden`（Golden 快照随镜像发布），`BUILD_SHA` 来自构建参数。

## 不用 Docker 本地运行

需要 Go 1.25+、Node 24 和 Postgres 16。

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
   (cd web && npm ci && npm run build)
   WEB_DIR=web/dist go run ./api/cmd/server      # 同时提供前端构建产物
   ```

   前端开发也可以 `cd web && npm run dev`（Vite 把 `/api` 代理到 `API_TARGET`，默认 `http://localhost:8080`）。

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

与 CI（`.github/workflows/ci.yml`）一致。Go 只检查 `./api` 和 `./daily`：根目录的 `./...` 会扫到 `web/node_modules` 里的 Go 包。

```sh
gofmt -l ./api ./daily                       # 必须没有输出
go vet ./api/... ./daily/...
TEST_DATABASE_URL='postgres://…/空库?sslmode=disable' go test ./api/... ./daily/...

cd web && npm ci && npm run lint && npm run type-check && npm test && npm run build
```

需要数据库的测试会在 `TEST_DATABASE_URL` 指向的库里建删 `dlpv9xx` 槽位，请用专门的空库。

迁移检查（CI 的 `migrations` 任务）：在空库上起 server 跑 public 迁移，`tenant reset` 两次并比较结构与数据，再重启 server 读台账。

```sh
go build -o bin/ ./api/cmd/server ./api/cmd/tenant
DATABASE_URL='postgres://…/空库?sslmode=disable' .github/scripts/check-migrations.sh
```

## E2E

`e2e/` 下的 Playwright 冒烟测试（测试名带 `SMOKE-n`），对已经起好的服务跑，不自己启动服务：

```sh
cd e2e
npm ci
npx playwright install chromium

# 对 docker compose（dev 租户）
E2E_TENANT=dev E2E_PASSWORD=pilot-pass npx playwright test

# 对预览槽位 dlpv1（默认租户），例如先执行上面的 tenant reset
E2E_PASSWORD=pilot-pass npx playwright test
```

| 变量 | 默认值 | 说明 |
|---|---|---|
| `E2E_BASE_URL` | `http://localhost:8080` | 被测服务地址 |
| `E2E_TENANT` | `dlpv1` | 测试账号的租户，账号为 `<role>@<E2E_TENANT>.test` |
| `E2E_PASSWORD` | 无（必填） | 测试账号密码，即 `TENANT_TEST_PASSWORD` |

写操作的用例（新建、提交、锁定）每次在 2100 年以后的随机业务日期新建自己的日报，不改快照里的日报，所以可以对同一个租户重复运行；台账用例只看快照的日期范围（2026-09-01 至 09-10）。报告格式用 `--reporter` 指定，CI 里是 `--reporter=list,html`，失败时上传 trace 和 HTML 报告。

## CI

`.github/workflows/ci.yml`：`go`、`web`、`migrations`、`e2e`（docker compose + `tenant reset dlpv1` + Playwright）、`image`（构建镜像；`dw/*` 分支的 PR 推送到 `ghcr.io/<owner>/<repo>:sha-<PR head>`）。
