import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Field } from './Field'
import { Input } from './Input'
import { Select } from './Select'

describe('Field', () => {
  it('labels its control', () => {
    render(
      <Field label="邮箱">
        <Input />
      </Field>,
    )
    expect(screen.getByRole('textbox', { name: '邮箱' })).toBeInTheDocument()
  })

  it('shows the error, marks the control invalid and describes it', () => {
    render(
      <Field label="邮箱" error="请输入邮箱" required>
        <Input />
      </Field>,
    )
    const input = screen.getByRole('textbox', { name: /邮箱/ })
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toBeRequired()
    expect(input).toHaveAccessibleDescription('请输入邮箱')
  })

  it('shows a hint when there is no error', () => {
    render(
      <Field label="工地" hint="可选">
        <Select options={[]} />
      </Field>,
    )
    const select = screen.getByRole('combobox', { name: '工地' })
    expect(select).toHaveAccessibleDescription('可选')
    expect(select).not.toHaveAttribute('aria-invalid')
  })
})
