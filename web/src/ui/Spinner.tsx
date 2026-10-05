import { cn } from './cn'

export type SpinnerSize = 'sm' | 'md'

export interface SpinnerProps {
  /** Accessible text announced to screen readers and shown next to the ring when `showLabel` is set. */
  label?: string
  size?: SpinnerSize
  /** Show the label as visible text, not just to assistive technology. */
  showLabel?: boolean
  className?: string
}

const ring: Record<SpinnerSize, string> = {
  sm: 'size-4 border-2',
  md: 'size-6 border-2',
}

/** Loading indicator. Always has an accessible name; never use it without one. */
export function Spinner({ label = '加载中', size = 'md', showLabel = false, className }: SpinnerProps) {
  return (
    <span role="status" aria-live="polite" className={cn('inline-flex items-center gap-2 text-muted', className)}>
      <span
        aria-hidden="true"
        className={cn('inline-block animate-spin rounded-full border-border-strong border-t-primary', ring[size])}
      />
      <span className={showLabel ? 'text-sm' : 'sr-only'}>{label}</span>
    </span>
  )
}
