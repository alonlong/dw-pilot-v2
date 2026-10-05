import type { ReactNode } from 'react'
import { cn } from './cn'

export interface PageHeaderProps {
  title: string
  description?: ReactNode
  /** The page's primary action(s), right-aligned. Keep to one primary Button. */
  actions?: ReactNode
  className?: string
}

/** The page's h1, an optional line of context, and its actions. One per page. */
export function PageHeader({ title, description, actions, className }: PageHeaderProps) {
  return (
    <div className={cn('flex flex-wrap items-end justify-between gap-4 pb-4', className)}>
      <div className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold text-fg">{title}</h1>
        {description && <div className="text-sm text-muted">{description}</div>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
