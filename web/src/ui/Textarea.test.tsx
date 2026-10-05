import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { Textarea } from './Textarea'

describe('Textarea', () => {
  it('renders a multi-line textbox with 3 rows by default', async () => {
    render(<Textarea aria-label="备注" />)
    const box = screen.getByRole('textbox', { name: '备注' })
    expect(box.tagName).toBe('TEXTAREA')
    expect(box).toHaveAttribute('rows', '3')
    await userEvent.type(box, '下雨停工')
    expect(box).toHaveValue('下雨停工')
  })

  it('supports invalid', () => {
    render(<Textarea aria-label="备注" invalid />)
    expect(screen.getByRole('textbox')).toHaveAttribute('aria-invalid', 'true')
  })
})
