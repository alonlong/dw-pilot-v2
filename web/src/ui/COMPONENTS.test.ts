// @vitest-environment node
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import * as ui from './index'

const doc = readFileSync(fileURLToPath(new URL('./COMPONENTS.md', import.meta.url)), 'utf8')
const sections = doc.split(/^## /m).slice(1)
const PROPS_HEADER = '| 属性 | 类型 | 默认值 | 必填 | 说明 |'

// Parts of a compound component documented under their parent's section.
const PARTS = new Set(['TableHead', 'TableBody', 'TableRow', 'TableHeaderCell', 'TableCell'])

describe('COMPONENTS.md', () => {
  it('documents exactly the components exported from @/ui', () => {
    const documented = sections.map((s) => s.split('\n')[0]!.trim()).sort()
    const exported = Object.keys(ui)
      .filter((k) => /^[A-Z]/.test(k) && !PARTS.has(k))
      .sort()
    expect(documented).toEqual(exported)
  })

  it.each(sections.map((s) => [s.split('\n')[0]!.trim(), s] as const))('%s follows the section format', (_name, body) => {
    expect(body).toMatch(/^用途：.+$/m)
    expect(body).toMatch(/^导入：`import \{ .+ \} from '@\/ui'`$/m)
    const order = ['### 属性', PROPS_HEADER, '### 变体', '### 示例', '```tsx'].map((h) => body.indexOf(h))
    expect(order.every((i) => i >= 0)).toBe(true)
    expect([...order].sort((a, b) => a - b)).toEqual(order)
  })
})
