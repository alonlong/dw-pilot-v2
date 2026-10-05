import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Badge, type BadgeTone } from './Badge'

describe('Badge', () => {
  it.each<[BadgeTone, string]>([
    ['draft', '未提交'],
    ['submitted', '已提交'],
    ['locked', '已锁定'],
    ['late', '迟交 2 天'],
  ])('%s carries its text, not only colour', (tone, text) => {
    render(<Badge tone={tone}>{text}</Badge>)
    const badge = screen.getByText(text)
    expect(badge).toHaveAttribute('data-tone', tone)
    expect(badge.className).toContain(`bg-status-${tone}-soft`)
  })

  it('hides the colour mark from assistive technology', () => {
    const { container } = render(<Badge tone="late">迟交 1 天</Badge>)
    expect(container.querySelector('[aria-hidden="true"]')).not.toBeNull()
  })
})
