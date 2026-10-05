# dw-pilot-v2 接口契约

前后端共用的 JSON 形状。设计背景见 `design.md` 第 5 节。字段名一律 camelCase；日期为 `YYYY-MM-DD`；时间为 RFC 3339（UTC）。

## 通用

- 前缀 `/api`；请求和响应都是 `application/json`。
- 会话：登录成功后服务端设置 Cookie `dl_session`（HttpOnly、SameSite=Lax、Path=/，`COOKIE_SECURE=1` 时带 Secure），有效期 12 小时。
- 写操作（POST / PUT / DELETE）若带 `Origin` 头，必须与请求的 Host 同源，否则 403 `bad_origin`。
- 错误：HTTP 状态码 + 

```json
{ "error": { "code": "negative_hours", "message": "工时不能为负数：张三=-1" } }
```

| code | HTTP | 含义 |
|---|---|---|
| `unauthenticated` | 401 | 没有会话或会话过期 |
| `bad_credentials` | 401 | 邮箱或密码错误（不区分是哪一个） |
| `forbidden` | 403 | 角色不允许 |
| `bad_origin` | 403 | 跨源写请求 |
| `not_found` | 404 | 资源不存在（含其他租户的资源） |
| `bad_request` | 400 | 参数格式错误 |
| `already_exists` | 409 | 同一工地同一天的日报已存在 |
| `already_submitted` | 409 | `daily.ErrAlreadySubmitted` |
| `locked` | 409 | 日报已锁定，不能改工时或提交 |
| `negative_hours` | 422 | `daily.ErrNegativeHours` |
| `empty_note` | 422 | `daily.ErrEmptyNote` |
| `note_too_long` | 422 | `daily.ErrNoteTooLong` |
| `tenant_outdated` | 503 | 租户 schema 的迁移版本落后于代码 |

## 类型

```ts
type Role = "admin" | "manager" | "field" | "finance";
type Status = "未提交" | "已提交" | "已锁定";   // 与 daily.StatusDraft 等常量一致

interface Me { id: number; email: string; name: string; role: Role; tenant: string; tenantName: string }
interface Job { id: string; name: string; address: string }

// GET /api/reports 的一行，对应 daily.Row，另加 id 和工地名
interface ReportRow {
  id: number; jobId: string; jobName: string;
  businessDate: string; status: Status; totalHours: number; daysLate: number;
}

interface CrewEntry { worker: string; hours: number }
interface Note { id: number; body: string; authorName: string; createdAt: string }

interface ReportDetail {
  id: number; jobId: string; jobName: string; businessDate: string;
  status: Status; submittedAt: string | null; locked: boolean;
  totalHours: number; daysLate: number;
  crew: CrewEntry[];          // 按 worker 升序
  notes: Note[];              // 新的在前（daily.LatestNotes 的顺序）
  can: { editCrew: boolean; submit: boolean; lock: boolean; addNote: boolean };  // 按当前用户角色和日报状态算好
}

interface SummaryRow { jobId: string; jobName: string; totalHours: number; lateCount: number }
```

## 接口

| 方法与路径 | 请求 | 成功响应 |
|---|---|---|
| `GET /healthz` | — | `200 {"status":"ok"}`；数据库不通时 `503 {"status":"db_unavailable"}` |
| `GET /api/version` | — | `{"sha":"<BUILD_SHA 或 dev>"}` |
| `POST /api/login` | `{"email","password"}` | `200 Me`，并设置 Cookie |
| `POST /api/logout` | — | `204` |
| `GET /api/me` | — | `Me` |
| `GET /api/jobs` | — | `{"jobs": Job[]}`（按 id 升序） |
| `GET /api/reports?job=&from=&to=` | 查询参数都可选；`from`、`to` 为日期，闭区间 | `{"rows": ReportRow[]}`（按业务日期、工地升序，即 `daily.Register` 的顺序） |
| `POST /api/reports` | `{"jobId","businessDate"}` | `201 ReportDetail` |
| `GET /api/reports/:id` | — | `ReportDetail` |
| `PUT /api/reports/:id/crew` | `{"crew": CrewEntry[]}`（整体替换；worker 去首尾空格后不能为空） | `ReportDetail` |
| `POST /api/reports/:id/submit` | — | `ReportDetail` |
| `POST /api/reports/:id/lock` | — | `ReportDetail` |
| `POST /api/reports/:id/notes` | `{"body"}` | `201 ReportDetail` |
| `GET /api/summary?from=&to=` | 可选日期范围 | `{"rows": SummaryRow[]}`（按工地 id 升序；只含有已提交日报或迟交的工地） |

## 权限

| 操作 | admin | manager | field | finance |
|---|---|---|---|---|
| 查看台账、详情、工地 | ✓ | ✓ | ✓ | ✓ |
| 新建日报、改工时、提交 | ✓ | | ✓ | |
| 锁定 | ✓ | ✓ | | |
| 追加备注 | ✓ | ✓ | ✓ | |
| 汇总 | ✓ | ✓ | | ✓ |

`ReportDetail.can`：`editCrew`、`submit` 还要求未锁定（`submit` 另要求未提交）；`lock` 要求未锁定；`addNote` 不受锁定影响。
