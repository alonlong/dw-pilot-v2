import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from './Button'

describe('Button', () => {
  it('defaults to type="button" and fires onClick', async () => {
    const onClick = vi.fn()
    render(<Button onClick={onClick}>保存</Button>)
    const btn = screen.getByRole('button', { name: '保存' })
    expect(btn).toHaveAttribute('type', 'button')
    await userEvent.click(btn)
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('applies the variant', () => {
    render(<Button variant="danger">删除</Button>)
    expect(screen.getByRole('button')).toHaveClass('bg-danger')
  })

  it('is disabled and busy while loading, keeping its label', async () => {
    const onClick = vi.fn()
    render(
      <Button loading onClick={onClick}>
        提交
      </Button>,
    )
    const btn = screen.getByRole('button', { name: /提交/ })
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByRole('status')).toHaveTextContent('处理中')
    await userEvent.click(btn)
    expect(onClick).not.toHaveBeenCalled()
  })
})
