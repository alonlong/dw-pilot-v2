import type { Ref, TextareaHTMLAttributes } from 'react'
import { cn } from './cn'
import { controlBase } from './controlStyles'
import { useFieldControl } from './fieldContext'

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean
  ref?: Ref<HTMLTextAreaElement>
}

/** Multi-line text entry, e.g. a report note. */
export function Textarea({ className, rows = 3, ...props }: TextareaProps) {
  const wired = useFieldControl(props)
  return <textarea rows={rows} className={cn(controlBase, 'py-2', className)} {...wired} />
}
