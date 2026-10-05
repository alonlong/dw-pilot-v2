import type { ReactNode } from 'react'
import { cn } from './cn'

export interface EmptyStateProps {
  title: string
  description?: ReactNode
  /** What the user can do next, e.g. a "新建日报" Button or "清除筛选". */
  action?: ReactNode
  className?: string
}

/** Shown in place of a list or table that has nothing to show. Says why, and what to do next. */
export function EmptyState({ title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center gap-2 rounded-md border border-dashed border-border-strong bg-surface px-6 py-10 text-center',
        className,
      )}
    >
      <p className="text-base font-medium text-fg">{title}</p>
      {description && <div className="max-w-md text-sm text-muted">{description}</div>}
      {action && <div className="pt-2">{action}</div>}
    </div>
  )
}
