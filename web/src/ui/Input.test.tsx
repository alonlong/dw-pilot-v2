import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { Input } from './Input'

describe('Input', () => {
  it('accepts typing', async () => {
    render(<Input aria-label="邮箱" />)
    const input = screen.getByRole('textbox', { name: '邮箱' })
    await userEvent.type(input, 'a@b.test')
    expect(input).toHaveValue('a@b.test')
  })

  it('marks invalid values with aria-invalid', () => {
    render(<Input aria-label="工时" invalid />)
    expect(screen.getByLabelText('工时')).toHaveAttribute('aria-invalid', 'true')
  })

  it('is not invalid by default', () => {
    render(<Input aria-label="工时" />)
    expect(screen.getByLabelText('工时')).not.toHaveAttribute('aria-invalid')
  })
})
