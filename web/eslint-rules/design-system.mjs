/**
 * Design-system rules for dw-pilot-v2 (docs/design.md §8).
 *
 * Business code (everything under src/ except src/ui/) builds its UI from the design
 * system and its tokens only. These rules make that a check rather than a convention:
 *
 *   no-raw-elements     <button>, <input>, <select>, <textarea>, <table> in JSX
 *   no-radix-import     importing @radix-ui/* (or the radix-ui umbrella package)
 *   ui-barrel-import    reaching into a src/ui file instead of importing from '@/ui'
 *   no-arbitrary-values Tailwind arbitrary values in class names: bg-[#123456], p-[13px], [color:red]
 *   no-hex-colors       hex colours in class names
 *   no-style-color      colours in a JSX style={{…}} object
 *
 * Class names are checked wherever they are written: inside a className attribute, and
 * inside calls to cn()/clsx() (class lists are often assembled in a variable first).
 *
 * eslint.config.mjs turns these on for src/** and off for src/ui/**, which is where the
 * raw elements, Radix and the token values are supposed to live.
 */
import path from 'node:path'

const RAW_ELEMENTS = new Map([
  ['button', 'Button'],
  ['input', 'Input'],
  ['select', 'Select'],
  ['textarea', 'Textarea'],
  ['table', 'Table'],
])

/**
 * `p-[13px]`, `bg-[#123456]`, `hover:min-w-[8rem]`, `[color:red]`.
 * Variants such as `data-[state=open]:` or `[&>svg]:` end with a colon and are not
 * values, so they are left alone. `bg-(--token)` is not an arbitrary value either: it
 * names a token.
 */
const ARBITRARY_RE = /(?:^|[\s:!])(?:-?[a-z][a-z0-9-]*-\[[^\]\s]*\]|\[[a-z-]+:[^\]\s]+\])(?![\w:-])/

/** `#fff`, `#0F5C8C`, `#0F5C8Ccc` — not `#1` or `#section-2`. */
const HEX_RE = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})(?![0-9a-zA-Z_-])/

const CLASS_ATTRS = new Set(['className', 'class'])
const CLASS_FUNCS = new Set(['cn', 'clsx', 'classnames', 'cx'])

/** Style properties that set a colour (or something that carries one). */
const COLOR_STYLE_PROP_RE = /color|background|fill|stroke|shadow|^border(?:Top|Right|Bottom|Left|Block|Inline)?$|^outline$|^caret|^accent/i

/** A colour literal written into a style value: hex, rgb(), hsl(), oklch() … */
const COLOR_VALUE_RE = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|oklch|oklab|lab|lch|color)\(/

/** Calls `check(node, text)` for every string literal / template chunk under `root`. */
function eachString(root, check) {
  const stack = [root]
  while (stack.length) {
    const node = stack.pop()
    if (!node || typeof node.type !== 'string') continue
    if (node.type === 'Literal' && typeof node.value === 'string') check(node, node.value)
    else if (node.type === 'TemplateElement') check(node, node.value.raw)
    for (const key of Object.keys(node)) {
      if (key === 'parent') continue
      const child = node[key]
      if (Array.isArray(child)) stack.push(...child)
      else if (child && typeof child === 'object' && typeof child.type === 'string') stack.push(child)
    }
  }
}

function calleeName(node) {
  if (node.callee.type === 'Identifier') return node.callee.name
  return null
}

/** Visitor that hands every class-name string in the file to `check`. */
function classStringVisitor(check) {
  return {
    JSXAttribute(node) {
      if (node.name.type === 'JSXIdentifier' && CLASS_ATTRS.has(node.name.name) && node.value) {
        eachString(node.value, check)
      }
    },
    CallExpression(node) {
      if (!CLASS_FUNCS.has(calleeName(node))) return
      // Inside a className attribute the attribute visitor already saw these strings.
      for (let p = node.parent; p; p = p.parent) {
        if (p.type === 'JSXAttribute' && p.name.type === 'JSXIdentifier' && CLASS_ATTRS.has(p.name.name)) return
      }
      for (const arg of node.arguments) eachString(arg, check)
    },
  }
}

const noRawElements = {
  meta: {
    type: 'problem',
    docs: { description: 'Use design-system components instead of raw form and table elements' },
    schema: [],
    messages: {
      raw: '<{{tag}}> is not allowed outside src/ui. Use <{{component}}> from @/ui.',
    },
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (node.name.type !== 'JSXIdentifier') return
        const component = RAW_ELEMENTS.get(node.name.name)
        if (component) context.report({ node, messageId: 'raw', data: { tag: node.name.name, component } })
      },
    }
  },
}

function isRadix(source) {
  return typeof source === 'string' && (source.startsWith('@radix-ui/') || source === 'radix-ui' || source.startsWith('radix-ui/'))
}

const noRadixImport = {
  meta: {
    type: 'problem',
    docs: { description: 'Only src/ui may use Radix primitives' },
    schema: [],
    messages: {
      radix:
        '"{{source}}" may only be imported inside src/ui. Use the wrapper from @/ui, or add one there (needs UX sign-off).',
    },
  },
  create(context) {
    const check = (node, source) => {
      if (source && isRadix(source.value)) context.report({ node, messageId: 'radix', data: { source: source.value } })
    }
    return {
      ImportDeclaration: (node) => check(node, node.source),
      ExportNamedDeclaration: (node) => check(node, node.source),
      ExportAllDeclaration: (node) => check(node, node.source),
      ImportExpression: (node) => node.source.type === 'Literal' && check(node, node.source),
    }
  },
}

const uiBarrelImport = {
  meta: {
    type: 'problem',
    docs: { description: "Import the design system from '@/ui', not from its files" },
    schema: [],
    messages: {
      deep: 'Import from "@/ui" instead of "{{source}}". Only what src/ui/index.ts exports is public.',
    },
  },
  create(context) {
    const filename = context.filename ?? context.getFilename()
    const check = (node, source) => {
      const value = source?.value
      if (typeof value !== 'string') return
      let deep = value.startsWith('@/ui/')
      if (!deep && value.startsWith('.') && filename) {
        const resolved = path.resolve(path.dirname(filename), value).split(path.sep).join('/')
        deep = /\/src\/ui\/.+/.test(resolved)
      }
      if (deep) context.report({ node, messageId: 'deep', data: { source: value } })
    }
    return {
      ImportDeclaration: (node) => check(node, node.source),
      ExportNamedDeclaration: (node) => check(node, node.source),
      ExportAllDeclaration: (node) => check(node, node.source),
    }
  },
}

const noArbitraryValues = {
  meta: {
    type: 'problem',
    docs: { description: 'Disallow Tailwind arbitrary values, which bypass the design tokens' },
    schema: [],
    messages: {
      arbitrary:
        'Arbitrary value "{{match}}" bypasses the design tokens. Use a token utility (see src/ui/tokens.css) or add a token there.',
    },
  },
  create(context) {
    return classStringVisitor((node, value) => {
      const m = ARBITRARY_RE.exec(value)
      if (m) context.report({ node, messageId: 'arbitrary', data: { match: m[0].trim().replace(/^[:!]/, '') } })
    })
  },
}

const noHexColors = {
  meta: {
    type: 'problem',
    docs: { description: 'Disallow hex colours in class names' },
    schema: [],
    messages: {
      hex: 'Hard-coded colour "{{match}}". Colours are tokens in src/ui/tokens.css (bg-surface, text-muted, bg-status-late …).',
    },
  },
  create(context) {
    return classStringVisitor((node, value) => {
      const m = HEX_RE.exec(value)
      if (m) context.report({ node, messageId: 'hex', data: { match: m[0] } })
    })
  },
}

const noStyleColor = {
  meta: {
    type: 'problem',
    docs: { description: 'Disallow colours in JSX style objects' },
    schema: [],
    messages: {
      prop: 'style.{{prop}} sets a colour outside the design tokens. Use a token utility class instead.',
      value: 'style.{{prop}} contains a colour literal. Use a token utility class instead.',
    },
  },
  create(context) {
    return {
      JSXAttribute(node) {
        if (node.name.type !== 'JSXIdentifier' || node.name.name !== 'style') return
        const expr = node.value?.type === 'JSXExpressionContainer' ? node.value.expression : null
        if (expr?.type !== 'ObjectExpression') return
        for (const prop of expr.properties) {
          if (prop.type !== 'Property') continue
          const name = prop.key.type === 'Identifier' ? prop.key.name : String(prop.key.value)
          if (COLOR_STYLE_PROP_RE.test(name)) {
            context.report({ node: prop, messageId: 'prop', data: { prop: name } })
            continue
          }
          let literal = false
          eachString(prop.value, (_n, text) => {
            if (COLOR_VALUE_RE.test(text)) literal = true
          })
          if (literal) context.report({ node: prop, messageId: 'value', data: { prop: name } })
        }
      },
    }
  },
}

export default {
  meta: { name: 'design-system' },
  rules: {
    'no-raw-elements': noRawElements,
    'no-radix-import': noRadixImport,
    'ui-barrel-import': uiBarrelImport,
    'no-arbitrary-values': noArbitraryValues,
    'no-hex-colors': noHexColors,
    'no-style-color': noStyleColor,
  },
}
