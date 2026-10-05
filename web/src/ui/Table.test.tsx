import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from './Table'

describe('Table', () => {
  it('renders a captioned table with column headers', () => {
    render(
      <Table caption="日报台账">
        <TableHead>
          <TableRow>
            <TableHeaderCell>工地</TableHeaderCell>
            <TableHeaderCell align="right">工时</TableHeaderCell>
          </TableRow>
        </TableHead>
        <TableBody>
          <TableRow>
            <TableCell>一号工地</TableCell>
            <TableCell align="right">8</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    )
    const table = screen.getByRole('table', { name: '日报台账' })
    const headers = within(table).getAllByRole('columnheader')
    expect(headers.map((h) => h.textContent)).toEqual(['工地', '工时'])
    expect(headers[0]).toHaveAttribute('scope', 'col')
    expect(within(table).getByRole('cell', { name: '8' })).toHaveClass('text-right')
  })
})
