import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Spinner } from './Spinner'

describe('Spinner', () => {
  it('has a default accessible name', () => {
    render(<Spinner />)
    expect(screen.getByRole('status')).toHaveTextContent('加载中')
    expect(screen.getByText('加载中')).toHaveClass('sr-only')
  })

  it('can show its label', () => {
    render(<Spinner label="正在加载日报" showLabel />)
    expect(screen.getByText('正在加载日报')).not.toHaveClass('sr-only')
  })
})
