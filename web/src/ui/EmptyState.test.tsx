import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Button } from './Button'
import { EmptyState } from './EmptyState'

describe('EmptyState', () => {
  it('explains and offers a next step', () => {
    render(<EmptyState title="还没有日报" description="新建第一份日报" action={<Button>新建日报</Button>} />)
    expect(screen.getByText('还没有日报')).toBeInTheDocument()
    expect(screen.getByText('新建第一份日报')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新建日报' })).toBeInTheDocument()
  })
})
