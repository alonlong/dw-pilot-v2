import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Button } from './Button'
import { Dialog } from './Dialog'

describe('Dialog', () => {
  it('renders nothing when closed', () => {
    render(<Dialog open={false} onOpenChange={() => {}} title="新建日报" />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('is a labelled, described dialog when open', () => {
    render(
      <Dialog open onOpenChange={() => {}} title="新建日报" description="每天一份" footer={<Button>创建</Button>}>
        <p>正文</p>
      </Dialog>,
    )
    const dialog = screen.getByRole('dialog', { name: '新建日报' })
    expect(dialog).toHaveAccessibleDescription('每天一份')
    expect(dialog).toHaveTextContent('正文')
    expect(screen.getByRole('button', { name: '创建' })).toBeInTheDocument()
  })

  it('asks to close on Escape', async () => {
    const onOpenChange = vi.fn()
    render(<Dialog open onOpenChange={onOpenChange} title="锁定？" />)
    await userEvent.keyboard('{Escape}')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
