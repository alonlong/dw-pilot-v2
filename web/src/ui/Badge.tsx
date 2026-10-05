import { cn } from './cn'

export type BadgeTone = 'draft' | 'submitted' | 'locked' | 'late' | 'neutral'

export interface BadgeProps {
  tone: BadgeTone
  /** The state's text. Required: colour is never the only signal (ui-design-system §1.1). */
  children: string
  className?: string
}

const tones: Record<BadgeTone, { box: string; mark: string }> = {
  draft: { box: 'bg-status-draft-soft text-status-draft-fg', mark: 'bg-status-draft' },
  submitted: { box: 'bg-status-submitted-soft text-status-submitted-fg', mark: 'bg-status-submitted' },
  locked: { box: 'bg-status-locked-soft text-status-locked-fg', mark: 'bg-status-locked' },
  late: { box: 'bg-status-late-soft text-status-late-fg', mark: 'bg-status-late' },
  neutral: { box: 'bg-surface-muted text-fg', mark: 'bg-border-strong' },
}

/** A short state label: 未提交, 已提交, 已锁定, 迟交 N 天. */
export function Badge({ tone, children, className }: BadgeProps) {
  const t = tones[tone]
  return (
    <span
      data-tone={tone}
      className={cn('inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-medium whitespace-nowrap', t.box, className)}
    >
      <span aria-hidden="true" className={cn('size-2 rounded-full', t.mark)} />
      {children}
    </span>
  )
}
