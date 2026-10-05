import type { HTMLAttributes, ReactNode, TdHTMLAttributes, ThHTMLAttributes } from 'react'
import { cn } from './cn'

export type CellAlign = 'left' | 'right' | 'center'

const alignClass: Record<CellAlign, string> = {
  left: 'text-left',
  right: 'text-right tabular-nums',
  center: 'text-center',
}

export interface TableProps extends HTMLAttributes<HTMLTableElement> {
  /** Accessible caption. Visually hidden unless `showCaption` is set. */
  caption: string
  showCaption?: boolean
  children: ReactNode
}

/** Data table. Compose with TableHead, TableBody, TableRow, TableHeaderCell and TableCell. */
export function Table({ caption, showCaption = false, className, children, ...rest }: TableProps) {
  return (
    <div className="w-full overflow-x-auto rounded-md border border-border bg-surface">
      <table className={cn('w-full border-collapse text-sm', className)} {...rest}>
        <caption className={showCaption ? 'px-4 py-2 text-left text-sm text-muted' : 'sr-only'}>{caption}</caption>
        {children}
      </table>
    </div>
  )
}

export function TableHead({ className, ...rest }: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={cn('bg-surface-muted', className)} {...rest} />
}

export function TableBody({ className, ...rest }: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody className={cn('divide-y divide-border', className)} {...rest} />
}

export function TableRow({ className, ...rest }: HTMLAttributes<HTMLTableRowElement>) {
  return <tr className={cn('align-middle', className)} {...rest} />
}

export interface TableHeaderCellProps extends ThHTMLAttributes<HTMLTableCellElement> {
  align?: CellAlign
}

export function TableHeaderCell({ align = 'left', className, ...rest }: TableHeaderCellProps) {
  return (
    <th
      scope="col"
      className={cn('border-b border-border px-4 py-2 text-xs font-semibold text-muted', alignClass[align], className)}
      {...rest}
    />
  )
}

export interface TableCellProps extends TdHTMLAttributes<HTMLTableCellElement> {
  align?: CellAlign
}

export function TableCell({ align = 'left', className, ...rest }: TableCellProps) {
  return <td className={cn('px-4 py-2 text-fg', alignClass[align], className)} {...rest} />
}
