// @vitest-environment node
import { RuleTester } from 'eslint'
import tseslint from 'typescript-eslint'
import { afterAll, describe, it } from 'vitest'
import plugin from './design-system.mjs'

RuleTester.afterAll = afterAll
RuleTester.describe = describe
RuleTester.it = it
RuleTester.itOnly = it.only

const tester = new RuleTester({
  languageOptions: {
    parser: tseslint.parser,
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
})

const PAGE = '/repo/web/src/pages/ReportsPage.tsx'
const rule = (name) => plugin.rules[name]

tester.run('no-raw-elements', rule('no-raw-elements'), {
  valid: [
    '<Button>保存</Button>',
    '<Input value={v} />',
    '<Table caption="x"><TableBody /></Table>',
    '<div><span /></div>',
    '<ui.button />',
  ],
  invalid: [
    { code: '<button type="button">保存</button>', errors: [{ messageId: 'raw', data: { tag: 'button', component: 'Button' } }] },
    { code: '<input />', errors: [{ messageId: 'raw' }] },
    { code: '<select><option /></select>', errors: [{ messageId: 'raw' }] },
    { code: '<textarea />', errors: [{ messageId: 'raw' }] },
    { code: '<table><tbody /></table>', errors: [{ messageId: 'raw' }] },
  ],
})

tester.run('no-radix-import', rule('no-radix-import'), {
  valid: ["import { Dialog } from '@/ui'", "import { useQuery } from '@tanstack/react-query'", "import x from 'radix-uix'"],
  invalid: [
    { code: "import * as D from '@radix-ui/react-dialog'", errors: [{ messageId: 'radix' }] },
    { code: "import { Dialog } from 'radix-ui'", errors: [{ messageId: 'radix' }] },
    { code: "export { Root } from '@radix-ui/react-label'", errors: [{ messageId: 'radix' }] },
    { code: "const m = import('@radix-ui/react-select')", errors: [{ messageId: 'radix' }] },
  ],
})

tester.run('ui-barrel-import', rule('ui-barrel-import'), {
  valid: [
    { code: "import { Button } from '@/ui'", filename: PAGE },
    { code: "import { api } from '@/api'", filename: PAGE },
    { code: "import { x } from './states'", filename: '/repo/web/src/app/Layout.tsx' },
  ],
  invalid: [
    { code: "import { Button } from '@/ui/Button'", filename: PAGE, errors: [{ messageId: 'deep' }] },
    { code: "import { cn } from '../ui/cn'", filename: PAGE, errors: [{ messageId: 'deep' }] },
  ],
})

tester.run('no-arbitrary-values', rule('no-arbitrary-values'), {
  valid: [
    '<div className="p-4 bg-surface text-muted" />',
    '<div className="data-[state=open]:bg-surface [&>svg]:size-4" />',
    '<div className="w-(--sidebar-width)" />',
    "const c = cn('px-3', active && 'bg-primary')",
    // Not a class list: ordinary strings are not inspected.
    "const label = '数组[0]-[1]'",
    '<p>{"text-[13px]"}</p>',
  ],
  invalid: [
    { code: '<div className="bg-[#123456]" />', errors: [{ messageId: 'arbitrary', data: { match: 'bg-[#123456]' } }] },
    { code: '<div className="p-4 p-[13px]" />', errors: [{ messageId: 'arbitrary', data: { match: 'p-[13px]' } }] },
    { code: '<div className="hover:min-w-[8rem]" />', errors: [{ messageId: 'arbitrary' }] },
    { code: '<div className="[color:red]" />', errors: [{ messageId: 'arbitrary' }] },
    { code: '<div className={`gap-2 ${x} mt-[3px]`} />', errors: [{ messageId: 'arbitrary' }] },
    { code: "<div className={cn('p-2', on && 'w-[120px]')} />", errors: [{ messageId: 'arbitrary' }] },
    { code: "const nav = cn('rounded-md', 'top-[7px]')", errors: [{ messageId: 'arbitrary' }] },
  ],
})

tester.run('no-hex-colors', rule('no-hex-colors'), {
  valid: [
    '<div className="bg-status-late text-status-late-fg" />',
    "const c = cn('text-fg')",
    // Hex-looking text outside class names is content, not styling.
    "const issue = '#123'",
    '<a href="#abc">x</a>',
  ],
  invalid: [
    { code: '<div className="text-#fff" />', errors: [{ messageId: 'hex', data: { match: '#fff' } }] },
    { code: '<div className="border #0F5C8C" />', errors: [{ messageId: 'hex' }] },
    { code: "const c = cn('fill-#0f5c8ccc')", errors: [{ messageId: 'hex' }] },
  ],
})

tester.run('no-style-color', rule('no-style-color'), {
  valid: [
    '<div style={{ width: 120 }} />',
    '<div style={{ gridTemplateColumns: "1fr 2fr" }} />',
    '<div style={s} />',
    '<div className="text-fg" />',
  ],
  invalid: [
    { code: '<div style={{ color: "red" }} />', errors: [{ messageId: 'prop', data: { prop: 'color' } }] },
    { code: '<div style={{ backgroundColor: v }} />', errors: [{ messageId: 'prop' }] },
    { code: '<div style={{ background: "#fff" }} />', errors: [{ messageId: 'prop' }] },
    { code: '<div style={{ borderColor: "#000", width: 2 }} />', errors: [{ messageId: 'prop' }] },
    { code: '<div style={{ boxShadow: "0 0 1px black" }} />', errors: [{ messageId: 'prop' }] },
    { code: '<div style={{ border: "1px solid #ccc" }} />', errors: [{ messageId: 'prop' }] },
    { code: '<div style={{ ["--accent"]: "rgb(1 2 3)" }} />', errors: [{ messageId: 'value' }] },
  ],
})
