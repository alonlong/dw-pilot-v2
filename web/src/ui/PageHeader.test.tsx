import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Button } from './Button'
import { PageHeader } from './PageHeader'

describe('PageHeader', () => {
  it('renders the h1, description and actions', () => {
    render(<PageHeader title="日报台账" description="全部日报" actions={<Button>新建日报</Button>} />)
    expect(screen.getByRole('heading', { level: 1, name: '日报台账' })).toBeInTheDocument()
    expect(screen.getByText('全部日报')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '新建日报' })).toBeInTheDocument()
  })

  it('works with a title only', () => {
    render(<PageHeader title="汇总" />)
    expect(screen.getByRole('heading')).toHaveTextContent('汇总')
    expect(screen.queryByRole('button')).toBeNull()
  })
})
