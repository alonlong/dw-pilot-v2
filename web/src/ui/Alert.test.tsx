import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Alert } from './Alert'

describe('Alert', () => {
  it('uses role=alert for danger', () => {
    render(
      <Alert tone="danger" title="加载失败">
        无法连接服务器
      </Alert>,
    )
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent('加载失败')
    expect(alert).toHaveTextContent('无法连接服务器')
  })

  it('uses role=status for the other tones and renders an action', () => {
    render(<Alert tone="info" action={<span>重试</span>}>已锁定</Alert>)
    const status = screen.getByRole('status')
    expect(status).toHaveAttribute('data-tone', 'info')
    expect(status).toHaveTextContent('重试')
  })
})
