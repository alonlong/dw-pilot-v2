import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Select } from './Select'

const options = [
  { value: 'J-1', label: '一号工地' },
  { value: 'J-2', label: '二号工地' },
]

describe('Select', () => {
  it('renders the options and the placeholder', () => {
    render(<Select aria-label="工地" options={options} placeholder="全部工地" />)
    const opts = screen.getAllByRole('option')
    expect(opts.map((o) => o.textContent)).toEqual(['全部工地', '一号工地', '二号工地'])
    expect(opts[0]).toHaveValue('')
  })

  it('omits the placeholder when not given', () => {
    render(<Select aria-label="工地" options={options} />)
    expect(screen.getAllByRole('option')).toHaveLength(2)
  })

  it('reports the chosen value', async () => {
    const onChange = vi.fn()
    render(<Select aria-label="工地" options={options} onChange={(e) => onChange(e.target.value)} />)
    await userEvent.selectOptions(screen.getByRole('combobox', { name: '工地' }), 'J-2')
    expect(onChange).toHaveBeenCalledWith('J-2')
  })
})
