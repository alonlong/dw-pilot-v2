# AGENTS.md

给在本仓库里写代码的 Agent（包括数字员工）看的规则。背景见 `docs/design.md`，接口契约见 `docs/api.md`。

## 目录

```
daily/                  领域包：迟交天数、工时合计、状态、备注校验（纯函数）
api/cmd/server/         HTTP 服务：/api/* + 静态前端
api/cmd/tenant/         预览槽位租户：check / seed / reset / drop
api/cmd/devseed/        本地开发租户 dev
api/internal/config/    环境变量
api/internal/db/        连接池、按租户切 search_path、迁移
api/internal/auth/      密码、会话、角色与权限                （R3）
api/internal/store/     日报读写（SQL ↔ daily.Report）
api/internal/httpapi/   路由与处理函数
api/internal/tenant/    快照读取与校验、租户建删             （R3）
api/migrations/         Goose 迁移：public/、tenant/          （R3）
api/testdata/golden/    虚构的 Golden 快照
web/src/ui/             设计系统：组件、token、COMPONENTS.md   （需 UX 确认）
web/src/pages/          业务页面
web/src/app/            路由、布局、登录状态、通用状态组件
web/src/api/            接口客户端与类型
web/eslint-rules/       设计系统 ESLint 规则                  （受保护）
e2e/                    Playwright 冒烟测试                    （受保护）
```

## 每次改动后必须跑的检查

与 CI（`.github/workflows/ci.yml`）一致，全部通过才算完成：

```sh
# Go（只检查 ./api 和 ./daily：根目录的 ./... 会扫到 web/node_modules 里的 Go 包）
gofmt -l ./api ./daily          # 必须没有输出
go vet ./api/... ./daily/...
TEST_DATABASE_URL='postgres://…/专用空库?sslmode=disable' go test ./api/... ./daily/...

# 前端（在 web/ 下）
npm ci
npm run lint                    # 含设计系统规则，零告警
npm run type-check
npm test
npm run build
```

没有 `TEST_DATABASE_URL` 时需要数据库的测试会跳过，CI 里一定会跑，所以本地也要配上再交付。改了迁移或快照时再跑一次迁移检查（空库）：

```sh
go build -o bin/ ./api/cmd/server ./api/cmd/tenant
DATABASE_URL='postgres://…/空库?sslmode=disable' .github/scripts/check-migrations.sh
```

改了页面或接口时，对本地服务跑 E2E（见 README「E2E」），`e2e/` 里的测试本身不要改。

## 代码规则

- **业务规则只放 `daily/`**：迟交天数、工时合计、状态文本、备注校验等只在 `daily/` 里实现。`api/` 的处理函数和 `web/` 的前端调用它的结果（经接口返回），不重复实现同样的计算。
- **时间由调用方传入**：`daily/` 里不调用 `time.Now()`；当前时间由服务端注入的时钟提供，测试里替换。
- **新增行为配表驱动测试**：Go 用 `[]struct{ name string; … }` + `t.Run`；处理函数的测试覆盖每个角色的允许和拒绝。
- **测试名带验收标准编号**：需求里的验收标准 AC-n 对应的测试，Go 写 `TestXxx_AC1`，TypeScript 写 `test("AC-1 …")`；一个 AC 至少一个测试。
- **前端只用设计系统**：业务代码只从 `@/ui` 导入组件（见 `web/src/ui/COMPONENTS.md`）；不直接写 `<button>`、`<input>`、`<select>`、`<textarea>`、`<table>`，不直接导入 `@radix-ui/*`，不用 Tailwind 任意值和十六进制颜色，`style` 里不写颜色。ESLint 设计规则会拦下，不要用 `eslint-disable` 绕过。缺组件时在需求里提出，由 UX 设计师确认后改 `web/src/ui/`。
- **界面文字为中文**；每个页面都有加载、空、错误三种状态。
- **注释与周围一致**：Go 代码注释用中文，`web/` 和 `e2e/` 的代码注释用英文。
- **租户与快照**：不要在代码里加绕过槽位名校验（`dlpv` 加数字）的开关；改了 `api/testdata/golden/tables/` 要按 README 更新 manifest。

## 不能改的路径

数字员工改动以下路径会被平台拒绝（`docs/design.md` 第 10 节）：

| 类别 | 路径 |
|---|---|
| 受保护 | `AGENTS.md`、`.github/**`、`Dockerfile`、`docker-compose.yml`、`go.mod`、`go.sum`、`web/package.json`、`web/package-lock.json`、`web/eslint-rules/**`、`e2e/**`、`.dw/**` |
| R3（需求不是 R3 时 G6 阻断） | `api/migrations/**`、`api/internal/auth/**`、`api/internal/tenant/**` |
| 需 UX 设计师确认 | `web/src/ui/**` |

- **不引入新依赖**：`go.mod`、`go.sum`、`web/package.json`、`web/package-lock.json` 都是受保护路径，需要新依赖时在需求里提出。
- **`e2e/` 是受保护测试**：由测试员生成、QA 审核后由平台写入；功能改动导致 E2E 失败时，说明原因并交回，不要改测试去迁就。
