# dw-pilot-v2 重新设计方案（DP2-01）

> **状态**：草案 v0.1，待技术负责人评审。
>
> **日期**：2026-10-05
>
> **依据**：`数字员工云平台-二期开发任务.md` v0.3 第一节（DP2）；技术设计文档 v0.6 4.3 节与附录 C（abby-pm 接入）；abby-pm 的 `api/cmd/tenant`、`api/internal/tenant`（`dw/mvp-prereqs` 分支）、`ui/`、`ui-design-system.md`。
>
> **决定（2026-10-05）**：按完整形态做（Postgres + schema-per-tenant + 租户命令）；新建仓库 `dw-pilot-v2`，不改 `dw-pilot-dailylog`。MVP OP-09 在 `dw-pilot-dailylog` 上验收通过后，平台的试点项目再切到本仓库。

## 1. 目标

二期的 E2E 测试、设计走查、UX 代码原型、R2 需求都要一个有前端、数据库、登录的项目。`dw-pilot-dailylog` 只有一个 Go 包，验证不了。本仓库是 abby-pm 的缩小版：

- **形态与 abby-pm 一致**：Go API + Postgres（schema-per-tenant）+ React SPA + 设计系统 + 登录与角色 + 租户命令 + 快照。平台为本仓库做的适配（租户重置、测试账号、设计系统读取）按 abby-pm 的契约设计，接 abby-pm 时只换配置。
- **业务保持简单**：施工日报，沿用 `dw-pilot-dailylog` 的 `daily/` 领域包和它的测试，3 个业务页面。
- **数据全部虚构**：快照放在仓库里，不需要脱敏和 CEO 确认。

不做：外部 OAuth、文件上传、邮件、外部系统同步。

## 2. 技术栈

与 abby-pm 对齐，版本取当前稳定版：

| 部分 | 选型 | abby-pm |
|---|---|---|
| 后端 | Go 1.25、Echo v4、pgx v5（不用 ORM）、Goose v3 | Go、Echo v4、pgx v5 + GORM、Goose |
| 密码 | `golang.org/x/crypto/argon2`（argon2id） | 不适用（Abby 登录） |
| 前端 | React 19、TypeScript、Vite、Tailwind CSS v4、Radix UI、react-router v7、TanStack Query v5 | 相同 |
| 前端测试 | Vitest + Testing Library | Vitest |
| E2E | Playwright（TypeScript） | Python CDP 脚本 |
| 数据库 | Postgres 16 | Postgres 17 |

## 3. 仓库结构

```
dw-pilot-v2/
├── AGENTS.md                 # 给编码 Agent 的规则（第 9 节）
├── README.md
├── go.mod                    # module example.com/dailylog（沿用，daily 包导入路径不变）
├── Dockerfile                # 一个镜像：server、tenant、devseed 三个命令 + 前端构建产物 + Golden 快照
├── docker-compose.yml        # 本地开发：postgres + app
├── daily/                    # 领域包，原样保留（纯函数，时间由调用方传入）
├── api/
│   ├── cmd/server/           # HTTP 服务：/api/* + 静态前端
│   ├── cmd/tenant/           # check / seed / reset / drop（第 6 节）
│   ├── internal/config/
│   ├── internal/db/          # 连接池、按租户切 search_path、迁移入口
│   ├── internal/auth/        # 密码、会话、角色与权限（R3）
│   ├── internal/store/       # 日报的读写（SQL ↔ daily.Report）
│   ├── internal/httpapi/     # 路由与处理函数
│   ├── internal/tenant/      # 快照读取与校验、租户建删（R3）
│   ├── migrations/public/    # public schema 迁移（R3）
│   ├── migrations/tenant/    # 租户 schema 迁移（R3）
│   └── testdata/golden/      # 虚构的 Golden 快照（第 6 节）
├── web/
│   ├── package.json
│   ├── eslint.config.mjs
│   ├── eslint-rules/design-system.mjs   # 设计系统规则（第 8 节）
│   ├── src/ui/               # 设计系统：组件 + token + COMPONENTS.md
│   ├── src/pages/            # 业务页面
│   ├── src/api/              # 接口客户端与类型
│   └── src/**/*.test.tsx
├── e2e/                      # Playwright；受保护测试目录（第 9 节）
├── docs/design.md            # 本文
└── .github/workflows/ci.yml
```

单个 Go module 放在根目录，`daily/` 的导入路径（`example.com/dailylog/daily`）不变。

## 4. 数据模型

### 4.1 public schema

| 表 | 字段 | 说明 |
|---|---|---|
| `tenant` | name（主键，`^[a-z][a-z0-9]{1,30}$`）、display_name、created_at | 租户登记；schema 名与 name 相同 |
| `app_user` | id、tenant、email（租户内唯一）、display_name、role、password_hash、disabled、created_at | 用户只属于一个租户 |
| `session` | id（随机 32 字节，存 SHA-256）、user_id、expires_at、created_at | 会话；删用户时级联删除 |

### 4.2 租户 schema

| 表 | 字段 | 说明 |
|---|---|---|
| `job` | id（如 `J-24001`）、name、address | 工地 |
| `report` | id、job_id、business_date、submitted_at（可空）、locked、created_at；`(job_id, business_date)` 唯一 | 日报 |
| `crew_hours` | report_id、worker、hours（≥ 0）；`(report_id, worker)` 主键 | 工人工时 |
| `report_note` | id、report_id、body（≤ 500 字符）、author_id、created_at | 追加的备注，按 id 保序 |

`store` 负责在 SQL 行与 `daily.Report` 之间转换；业务规则（迟交天数、工时合计、状态文本、备注校验）只在 `daily/` 里实现，处理函数不重复实现。当前时间由服务端注入的时钟提供（测试可替换），`daily/` 里不调用 `time.Now()`。

## 5. 接口与页面

### 5.1 角色与权限

| 角色 | 代码 | 能做什么 |
|---|---|---|
| 管理员 | `admin` | 全部操作 |
| 项目经理 | `manager` | 查看全部；锁定日报；查看汇总 |
| 现场 | `field` | 新建日报、编辑工时、提交、追加备注；不能锁定 |
| 财务 | `finance` | 只读：台账、详情、汇总 |

已锁定的日报任何角色都不能改工时、不能再提交；仍可追加备注（与 `dw-pilot-dailylog` 的语义一致）。

### 5.2 接口

全部在 `/api` 下，JSON；除登录和健康检查外都要会话 Cookie（HttpOnly、SameSite=Lax）；写操作校验 `Origin`。

| 方法与路径 | 角色 | 说明 |
|---|---|---|
| `GET /healthz` | — | 健康检查，含数据库连通性 |
| `GET /api/version` | — | 构建版本（`BUILD_SHA`） |
| `POST /api/login`、`POST /api/logout`、`GET /api/me` | — | 登录（租户由账号决定）、登出、当前用户 |
| `GET /api/jobs` | 全部 | 工地列表 |
| `GET /api/reports?job=&from=&to=` | 全部 | 台账（`daily.Register` 的行） |
| `POST /api/reports` | admin、field | 新建日报（job、business_date） |
| `GET /api/reports/:id` | 全部 | 详情：工时、备注（新的在前）、状态、迟交天数 |
| `PUT /api/reports/:id/crew` | admin、field | 覆盖工时 |
| `POST /api/reports/:id/submit` | admin、field | 提交 |
| `POST /api/reports/:id/lock` | admin、manager | 锁定 |
| `POST /api/reports/:id/notes` | admin、manager、field | 追加备注 |
| `GET /api/summary?from=&to=` | admin、manager、finance | 按工地汇总工时（`HoursByJob`）与迟交份数（`LateCountByJob`） |

错误统一为 `{"error": {"code": "...", "message": "..."}}`，`code` 取自 `daily` 的错误（如 `already_submitted`、`negative_hours`、`note_too_long`）和权限错误（`forbidden`、`unauthenticated`）。

### 5.3 页面

| 路由 | 页面 | 主要内容 |
|---|---|---|
| `/login` | 登录 | 邮箱、密码；错误提示 |
| `/reports` | 日报台账 | 按工地、日期筛选；表格列：工地、业务日期、状态、工时合计、迟交天数（迟交时显示"迟交 N 天"标记）；新建日报（field、admin） |
| `/reports/:id` | 日报详情 | 工时编辑表、提交、锁定（按角色显示）、备注列表与追加；已锁定为只读状态 |
| `/summary` | 汇总 | 按工地的总工时与迟交份数；日期范围 |

每个页面都要有加载、空、错误三种状态。界面文字为中文。

## 6. 租户、快照与测试账号

与 abby-pm `cmd/tenant` 的契约一致：

```
tenant check -snapshot api/testdata/golden
tenant seed  -tenant dlpv1 -snapshot <dir> [-accounts <file>]
tenant reset -tenant dlpv1 -snapshot <dir> [-accounts <file>]
tenant drop  -tenant dlpv1
```

- **只接受槽位租户**：名字必须是 `dlpv` 加数字（abby-pm 为 `pmpv`），其他名字一律拒绝，没有开关。开发用租户 `dev` 只由 `docker compose` 的初始化脚本建立，不经这个命令。
- **reset** = 删 schema（如存在）→ 建 schema 并跑租户迁移 → 按快照导入 → 写入测试账号。重复执行结果一致。
- **快照格式**沿用 abby-pm：`manifest.json`（format、name、description、tenant_migration、tables[]：name、file、rows、sha256）+ `tables/<table>.jsonl`。导入前校验行数和 SHA-256，导入后回读行数。
- **Golden 快照**（`api/testdata/golden`，全部虚构）：3 个工地，约 30 份日报，覆盖未提交、按时提交、迟交 1～3 天、已锁定、有备注、无工时等情况；业务日期固定在 2026-09，结果不随运行日期变化。
- **测试账号**：账号文件格式 `{"tenant": "dlpv1", "accounts": [{"email", "name", "role"}]}`；不给 `-accounts` 时按四种角色生成默认账号（`admin@dlpv1.test` 等），密码取环境变量 `TENANT_TEST_PASSWORD`（没有时拒绝执行）。平台在验收页显示账号，密码从平台的 Secret 读取。

## 7. 构建、运行与预览契约

- **镜像**：`Dockerfile` 三阶段：Node 构建前端 → Go 构建 `server`、`tenant`、`devseed` → 运行镜像（distroless、非 root，只含三个二进制、`web/dist`（`/app/web`）和 Golden 快照（`/app/golden`））。三个命令都在 PATH 里，只设 `CMD ["server"]`，`docker run <镜像> tenant reset …` 直接调用租户命令。构建时 `BUILD_SHA` 作为参数传入。
- **配置**（环境变量）：`DATABASE_URL`、`ADDR`（默认 `:8080`）、`WEB_DIR`、`BUILD_SHA`、`COOKIE_SECURE`、`TENANT_TEST_PASSWORD`（只给 `tenant` 用）。
- **服务启动时**跑 public 迁移；租户迁移由 `tenant seed/reset` 跑，服务启动时检查每个已登记租户的迁移版本，落后时拒绝该租户的请求并记日志（不自动迁移，避免多副本竞争）。
- **平台的预览契约**（DP2-06 在平台侧实现）：
  1. 用仓库自带的 `Dockerfile` 构建 PR head 的镜像，记录 digest；
  2. 槽位 = 一个 `dlpv<N>` 租户 + 一个运行该镜像的容器（共用一个 Postgres）；
  3. 重置数据：用同一个镜像执行 `tenant reset -tenant dlpv<N> -snapshot /app/golden`（快照随镜像发布，版本与代码一致）；
  4. 冒烟：`/healthz` 200；`/api/version` 等于 PR head；用测试账号登录后 `/api/reports` 的行数等于快照里的日报数。

  与 abby-pm 的差别只在第 1、3 步的触发方式（abby-pm 是 workflow dispatch 和 Cloud Run Job），由平台的槽位适配器按项目配置选择。

## 8. 设计系统

- **位置**：`web/src/ui/`。业务代码只能从 `@/ui` 导入组件。
- **token**：`web/src/ui/tokens.css` 定义颜色（含状态色：未提交、已提交、已锁定、迟交）、间距、圆角、字号，用 CSS 变量并接入 Tailwind v4 的 `@theme`。
- **组件**（约 12 个）：Button、Input、Textarea、Select、Field（标签 + 错误）、Table、Badge（状态）、Alert、Dialog、PageHeader、EmptyState、Spinner。每个组件有单测。
- **组件清单**：`web/src/ui/COMPONENTS.md`，列出每个组件的用途、属性、变体和示例，供 UX 设计员和设计走查读取（AP-03 对 abby-pm 产出同样格式的清单）。
- **ESLint 设计规则**（`web/eslint-rules/design-system.mjs`）：在 `src/ui/` 以外禁止：直接使用 `<button>`、`<input>`、`<select>`、`<textarea>`、`<table>`；直接导入 `@radix-ui/*`；Tailwind 任意值（`bg-[#…]`、`p-[13px]`）和十六进制颜色；`style` 属性里的颜色。CI 里零告警才算通过。

## 9. 测试、CI 与 AGENTS.md

- **Go**：`gofmt`、`go vet`、`go test`（范围一律是 `./api/... ./daily/...`：根目录的 `./...` 会扫到 `web/node_modules` 里的 Go 包）；需要数据库的测试用环境变量 `TEST_DATABASE_URL`，没有时跳过（CI 里有 Postgres 服务，一定会跑）。处理函数测试覆盖每个角色的允许和拒绝。
- **前端**：`npm run lint`（含设计规则）、`npm run type-check`、`npm test`、`npm run build`。
- **迁移检查**：在空库上 `up` 全部迁移，再对一个租户 `reset` 两次比较结果（`.github/scripts/check-migrations.sh`）。
- **E2E**：`e2e/` 下的 Playwright 测试，对 `docker compose` 起的服务跑：登录、台账、详情的冒烟用例。CI 里单独一个任务。
- **镜像**：每个 PR 构建镜像；`dw/*` 分支的 PR 推送到 GHCR（tag `sha-<head>`）（对应 abby-pm 的 PM-03）。
- **AGENTS.md** 写明：目录与命令；改动后必须跑的检查；表驱动测试；业务规则只放 `daily/`；时间由调用方传入；前端只用设计系统；不改受保护路径。

## 10. 平台侧的路径规则

| 类别 | 路径 |
|---|---|
| 允许 | `daily/**`、`api/**`、`web/src/**`（除下面的 R3 和受保护路径） |
| R3（命中而需求不是 R3 时 G6 阻断） | `api/migrations/**`、`api/internal/auth/**`、`api/internal/tenant/**` |
| 受保护（数字员工改动即被拒绝） | `AGENTS.md`、`.github/**`、`Dockerfile`、`docker-compose.yml`、`go.mod`、`go.sum`、`web/package.json`、`web/package-lock.json`、`web/eslint-rules/**`、`e2e/**`（受保护测试，经测试员生成、QA 审核后由平台写入）、`.dw/**` |
| 需 UX 设计师确认 | `web/src/ui/**`（新增或修改设计系统组件要在设计稿里标注，G2 检查） |

## 11. 实施顺序

| 任务 | 内容 | 依赖 |
|---|---|---|
| DP2-02 | 后端、数据库、租户命令、Golden 快照 | 本文 |
| DP2-03 | 前端与设计系统（可先对接口桩开发） | 本文 |
| DP2-04 | 登录与角色 | DP2-02 |
| DP2-05 | Dockerfile、docker-compose、CI、E2E 冒烟、AGENTS.md；推到 GitHub 并配置规则集 | DP2-02～04 |
| DP2-06 | 平台适配：新增项目 `dw-pilot-v2`（与 `dw-pilot` 并存）、路径规则、沙箱镜像加 Node 与浏览器、槽位适配器支持"仓库自带 Dockerfile + tenant reset" | DP2-05 |
| DP2-07 | 在 `dw-pilot-v2` 上重跑 MVP 端到端 | DP2-06 |

平台侧新增独立项目 `dw-pilot-v2`，不改现有 `dw-pilot`，所以 DP2-06 可以在 OP-09 之前合入平台，不影响 MVP 验收；OP-09 通过后再把默认试点切过来。

## 12. 已决定（2026-10-05，DP2-05）

| # | 事项 | 决定 |
|---|---|---|
| 1 | 仓库公开还是私有 | 公开（免费账号的私有仓库不能用规则集，与 `dw-pilot-dailylog` 相同）；仓库里只有虚构数据 |
| 2 | 镜像仓库 | GHCR（`ghcr.io/<owner>/dw-pilot-v2:sha-<PR head>`，`dw/*` 分支的 PR 由 CI 推送）；GCP 就绪后改推 Artifact Registry |
| 3 | 推到 GitHub 的时机 | DP2-05 完成后一次性推送并配置规则集；之前只在本地开发 |
