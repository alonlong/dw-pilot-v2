import type { ReactNode } from 'react'
import { cn } from './cn'

export type AlertTone = 'info' | 'success' | 'warning' | 'danger'

export interface AlertProps {
  tone?: AlertTone
  title?: string
  children?: ReactNode
  /** e.g. a "重试" Button. */
  action?: ReactNode
  className?: string
}

const tones: Record<AlertTone, string> = {
  info: 'bg-info-soft text-info-fg border-info-fg',
  success: 'bg-success-soft text-success-fg border-success-fg',
  warning: 'bg-warning-soft text-warning-fg border-warning-fg',
  danger: 'bg-danger-soft text-danger-fg border-danger',
}

/** Inline message. `danger` is announced immediately (role="alert"); the rest politely (role="status"). */
export function Alert({ tone = 'info', title, children, action, className }: AlertProps) {
  return (
    <div
      role={tone === 'danger' ? 'alert' : 'status'}
      data-tone={tone}
      className={cn('flex items-start justify-between gap-4 rounded-md border-l-4 px-4 py-3 text-sm', tones[tone], className)}
    >
      <div className="flex flex-col gap-1">
        {title && <p className="font-semibold">{title}</p>}
        {children && <div>{children}</div>}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
