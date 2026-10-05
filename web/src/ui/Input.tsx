import type { InputHTMLAttributes, Ref } from 'react'
import { cn } from './cn'
import { controlBase } from './controlStyles'
import { useFieldControl } from './fieldContext'

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  /** Marks the value as invalid (red border, aria-invalid). Inside a Field with an error this is automatic. */
  invalid?: boolean
  ref?: Ref<HTMLInputElement>
}

/** Single-line text, number, date, email or password entry. */
export function Input({ className, ...props }: InputProps) {
  const wired = useFieldControl(props)
  return <input className={cn(controlBase, 'h-10', className)} {...wired} />
}
