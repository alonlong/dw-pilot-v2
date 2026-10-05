# 设计系统组件清单

> 供 UX 设计员、设计走查和编码 Agent 读取。业务代码只能从 `@/ui` 导入这些组件（`eslint-rules/design-system.mjs` 强制）。
>
> **格式约定（机器解析用）**：每个组件一个二级标题 `## 组件名`；其下固定为 `用途：`、`导入：` 两行，然后依次是 `### 属性`（表格，列固定为 属性 | 类型 | 默认值 | 必填 | 说明）、`### 变体`（列表；没有时写"无"）、`### 示例`（一个 `tsx` 代码块）。组件之外的说明只出现在第一个二级标题之前。

令牌定义在 `src/ui/tokens.css`（Tailwind v4 `@theme`），默认调色板已清空，只能用下面的语义令牌：

| 类别 | 令牌（Tailwind 工具类前缀后的名字） |
|---|---|
| 表面与文字 | `canvas`、`surface`、`surface-muted`、`border`、`border-strong`、`fg`、`muted`、`overlay`、`white` |
| 交互 | `primary`、`primary-hover`、`primary-fg`、`focus` |
| 反馈 | `danger`、`danger-hover`、`danger-soft`、`danger-fg`、`warning-soft`、`warning-fg`、`success-soft`、`success-fg`、`info-soft`、`info-fg` |
| 日报状态 | `status-draft`（未提交）、`status-submitted`（已提交）、`status-locked`（已锁定）、`status-late`（迟交），各带 `-soft`（底色）和 `-fg`（文字） |
| 间距 | Tailwind 4px 刻度（`--spacing: 0.25rem`），如 `p-4` = 16px；禁止任意值 |
| 圆角 | `rounded-sm`、`rounded-md`、`rounded-lg`、`rounded-full` |
| 字号 | `text-xs`、`text-sm`、`text-base`、`text-lg`、`text-xl` |
| 阴影 | `shadow-overlay`（只用于浮层） |

## Button

用途：页面里唯一的按钮。触发操作或提交表单；一屏只放一个 `primary`。
导入：`import { Button } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| variant | `'primary' \| 'secondary' \| 'danger' \| 'ghost'` | `'primary'` | 否 | 视觉层级 |
| size | `'sm' \| 'md'` | `'md'` | 否 | 高度 32px / 40px |
| loading | `boolean` | `false` | 否 | 显示转圈、禁用按钮、设置 `aria-busy`，文字保留 |
| type | `'button' \| 'submit' \| 'reset'` | `'button'` | 否 | 默认不提交表单，提交按钮要显式写 `type="submit"` |
| disabled | `boolean` | `false` | 否 | 禁用 |
| children | `ReactNode` | — | 是 | 按钮文字（动词，如"保存工时"） |
| ...rest | `ButtonHTMLAttributes` | — | 否 | 透传给 `<button>`（如 `onClick`、`aria-label`） |

### 变体

- `primary`：主操作（新建日报、提交日报、保存工时）
- `secondary`：次要操作（锁定、取消、添加工人、重试）
- `danger`：破坏性操作
- `ghost`：低强调操作（清除筛选、行内删除）

### 示例

```tsx
<Button onClick={() => setOpen(true)}>新建日报</Button>
<Button type="submit" loading={save.isPending}>保存工时</Button>
<Button variant="secondary" size="sm">重试</Button>
```

## Input

用途：单行输入：文本、数字、日期、邮箱、密码。
导入：`import { Input } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| invalid | `boolean` | `false` | 否 | 红色边框并设置 `aria-invalid`；放在有 `error` 的 Field 里时自动生效 |
| type | `string` | `'text'` | 否 | `text`、`number`、`date`、`email`、`password` 等 |
| ...rest | `InputHTMLAttributes` | — | 否 | 透传给 `<input>`（`value`、`onChange`、`aria-label` 等） |

### 变体

无

### 示例

```tsx
<Field label="邮箱" required>
  <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
</Field>
<Input aria-label="第 1 行工时" type="number" min={0} step={0.5} invalid={hasError} />
```

## Textarea

用途：多行输入，如日报备注。
导入：`import { Textarea } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| rows | `number` | `3` | 否 | 可见行数 |
| invalid | `boolean` | `false` | 否 | 同 Input |
| ...rest | `TextareaHTMLAttributes` | — | 否 | 透传给 `<textarea>` |

### 变体

无

### 示例

```tsx
<Field label="追加备注" hint={`${length} / 500`} error={error}>
  <Textarea value={body} onChange={(e) => setBody(e.target.value)} />
</Field>
```

## Select

用途：从短列表中选一个值（工地等）。原生下拉框：可访问、手机上好用、不需要浮层。
导入：`import { Select } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| options | `{ value: string; label: string }[]` | — | 是 | 选项 |
| placeholder | `string` | — | 否 | 值为空的第一项，如"全部工地""请选择工地"；必选时也可用它提示 |
| invalid | `boolean` | `false` | 否 | 同 Input |
| ...rest | `SelectHTMLAttributes` | — | 否 | 透传给 `<select>`（`value`、`onChange` 等），不接受 `children` |

### 变体

无

### 示例

```tsx
<Field label="工地">
  <Select
    value={job}
    onChange={(e) => setJob(e.target.value)}
    placeholder="全部工地"
    options={jobs.map((j) => ({ value: j.id, label: `${j.name}（${j.id}）` }))}
  />
</Field>
```

## Field

用途：标签 + 控件 + 提示/错误。自动用 id 关联三者，屏幕阅读器会一起读出；有 `error` 时控件自动 `aria-invalid`。
导入：`import { Field } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| label | `string` | — | 是 | 标签文字 |
| error | `string` | — | 否 | 错误文字，显示在控件下方（红色） |
| hint | `string` | — | 否 | 提示文字，没有错误时显示 |
| required | `boolean` | `false` | 否 | 标签后显示 `*`，控件设置 `required` |
| children | `ReactNode` | — | 是 | 恰好一个 Input、Textarea 或 Select |
| className | `string` | — | 否 | 外层布局类 |

### 变体

无

### 示例

```tsx
<Field label="业务日期" error={missing.date} required>
  <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
</Field>
```

## Table

用途：数据表格（台账、工时、汇总）。由 `Table`、`TableHead`、`TableBody`、`TableRow`、`TableHeaderCell`、`TableCell` 组合；窄屏时横向滚动。
导入：`import { Table, TableHead, TableBody, TableRow, TableHeaderCell, TableCell } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| caption | `string` | — | 是 | 表格的可访问名称（Table） |
| showCaption | `boolean` | `false` | 否 | 显示标题；默认只给屏幕阅读器（Table） |
| align | `'left' \| 'right' \| 'center'` | `'left'` | 否 | 单元格对齐；`right` 同时使用等宽数字，数字列用它（TableHeaderCell、TableCell） |
| ...rest | 对应 HTML 属性 | — | 否 | 透传 |

### 变体

无

### 示例

```tsx
<Table caption="日报台账，共 3 份">
  <TableHead>
    <TableRow>
      <TableHeaderCell>工地</TableHeaderCell>
      <TableHeaderCell align="right">工时合计</TableHeaderCell>
    </TableRow>
  </TableHead>
  <TableBody>
    <TableRow>
      <TableCell>滨江一号</TableCell>
      <TableCell align="right">24.5</TableCell>
    </TableRow>
  </TableBody>
</Table>
```

## Badge

用途：短状态标签：未提交、已提交、已锁定、迟交 N 天。必须有文字，颜色不是唯一信号（前面的色点对辅助技术隐藏）。
导入：`import { Badge } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| tone | `'draft' \| 'submitted' \| 'locked' \| 'late' \| 'neutral'` | — | 是 | 状态色 |
| children | `string` | — | 是 | 状态文字，只接受字符串 |
| className | `string` | — | 否 | 外加类 |

### 变体

- `draft`：未提交（灰）
- `submitted`：已提交（绿）
- `locked`：已锁定（蓝灰）
- `late`：迟交（琥珀），文字写"迟交 N 天"或"迟交 N 份"
- `neutral`：其他中性标记

### 示例

```tsx
<Badge tone="submitted">已提交</Badge>
<Badge tone="late">{`迟交 ${daysLate} 天`}</Badge>
```

## Alert

用途：行内消息：加载失败、只读原因、无权限、操作错误。`danger` 立即播报（`role="alert"`），其余礼貌播报（`role="status"`）。
导入：`import { Alert } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| tone | `'info' \| 'success' \| 'warning' \| 'danger'` | `'info'` | 否 | 语气 |
| title | `string` | — | 否 | 粗体标题 |
| children | `ReactNode` | — | 否 | 正文 |
| action | `ReactNode` | — | 否 | 右侧操作，如"重试"按钮 |
| className | `string` | — | 否 | 外加类 |

### 变体

- `info`：说明（如"只读：已锁定"）
- `success`：操作成功
- `warning`：需要注意（如"无权限"）
- `danger`：错误（加载失败、接口错误）

### 示例

```tsx
<Alert tone="danger" title="日报加载失败" action={<Button variant="secondary" size="sm" onClick={retry}>重试</Button>}>
  {errorMessage(error)}
</Alert>
```

## Dialog

用途：模态对话框：新建日报、提交/锁定确认。焦点限制在框内，Esc 关闭，标题即可访问名称。只支持受控用法。
导入：`import { Dialog } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| open | `boolean` | — | 是 | 是否打开 |
| onOpenChange | `(open: boolean) => void` | — | 是 | 打开状态变化（Esc、点遮罩、关闭） |
| title | `string` | — | 是 | 标题 |
| description | `string` | — | 否 | 标题下的说明，作为可访问描述 |
| children | `ReactNode` | — | 否 | 正文（表单等） |
| footer | `ReactNode` | — | 否 | 底部右对齐的按钮 |
| className | `string` | — | 否 | 内容框外加类 |

### 变体

无

### 示例

```tsx
<Dialog
  open={confirm}
  onOpenChange={setConfirm}
  title="锁定这份日报？"
  description="锁定后任何人都不能再修改工时或提交，仍可追加备注。"
  footer={
    <>
      <Button variant="secondary" onClick={() => setConfirm(false)}>取消</Button>
      <Button loading={lock.isPending} onClick={() => lock.mutate()}>确认锁定</Button>
    </>
  }
/>
```

## PageHeader

用途：页面的 h1、一行背景说明和页面主操作。每页一个。
导入：`import { PageHeader } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| title | `string` | — | 是 | 页面标题（h1） |
| description | `ReactNode` | — | 否 | 标题下的说明，可放 Badge |
| actions | `ReactNode` | — | 否 | 右侧操作，最多一个 primary |
| className | `string` | — | 否 | 外加类 |

### 变体

无

### 示例

```tsx
<PageHeader title="日报台账" description="按业务日期排列的全部日报。" actions={<Button>新建日报</Button>} />
```

## EmptyState

用途：列表或表格没有内容时代替它显示：说明为什么空、下一步做什么。
导入：`import { EmptyState } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| title | `string` | — | 是 | 一句话说明 |
| description | `ReactNode` | — | 否 | 补充说明 |
| action | `ReactNode` | — | 否 | 下一步操作，如"新建日报""清除筛选" |
| className | `string` | — | 否 | 外加类 |

### 变体

无

### 示例

```tsx
<EmptyState
  title="没有符合筛选条件的日报"
  description="换一个工地或日期范围试试。"
  action={<Button variant="secondary" onClick={clear}>清除筛选</Button>}
/>
```

## Spinner

用途：加载指示。总有可访问名称（`role="status"`），不能不带说明地使用。
导入：`import { Spinner } from '@/ui'`

### 属性

| 属性 | 类型 | 默认值 | 必填 | 说明 |
|---|---|---|---|---|
| label | `string` | `'加载中'` | 否 | 播报文字，如"正在加载日报" |
| showLabel | `boolean` | `false` | 否 | 同时显示文字；默认只给屏幕阅读器 |
| size | `'sm' \| 'md'` | `'md'` | 否 | 16px / 24px |
| className | `string` | — | 否 | 外加类 |

### 变体

- `sm`：按钮内、行内
- `md`：页面或区块加载

### 示例

```tsx
<Spinner label="正在加载日报" showLabel />
```
