# DEV-69 设计说明：工时统一保留一位小数

## 变更点

- 只改 `web/src/app/format.ts` 的 `formatHours`：`Intl.NumberFormat('zh-CN', { minimumFractionDigits: 1, maximumFractionDigits: 1 })`。
  - 64 → `64.0`；120.5 → `120.5`；7.25 → `7.3`（四舍五入）；1280 → `1,280.0`（保留千分位）；0 → `0.0`。
- 页面结构、组件、文案不变。汇总页「总工时（小时）」列已是 `align="right"`，统一一位小数后小数点自然对齐。
- 台账（`/reports`）和日报详情（`/reports/:id`）同样经 `formatHours` 显示工时，随之统一，不需单独设计；走查时顺带确认这两页工时为一位小数即可。
- 不改接口返回值、不改录入步长（0.5）。

## 原型

`/__prototypes/DEV-69/?screen=S1&state=normal|empty|loading|error|noPermission`

normal 示例数据刻意混排整数、半小时、两位小数和千位数（1,280.0 / 120.5 / 64.0 / 7.3 / 0.0），便于核对对齐效果。

## 实现提示

- AC-1 测试建议：`test("AC-1 汇总页整数工时显示一位小数")`，mock 汇总接口返回 `totalHours: 64`，断言单元格文字为 `64.0`。
- 现有测试若断言了旧格式（如 `64`），需要同步更新。
