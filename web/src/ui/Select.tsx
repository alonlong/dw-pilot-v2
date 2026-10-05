import type { Ref, SelectHTMLAttributes } from 'react'
import { cn } from './cn'
import { controlBase } from './controlStyles'
import { useFieldControl } from './fieldContext'

export interface SelectOption {
  value: string
  label: string
}

export interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'children'> {
  options: SelectOption[]
  /** First option with an empty value, e.g. "全部工地". Omit when a choice is mandatory. */
  placeholder?: string
  invalid?: boolean
  ref?: Ref<HTMLSelectElement>
}

/**
 * Pick one value from a short list. A native select: it is accessible, works on phones
 * on site, and needs no portal.
 */
export function Select({ options, placeholder, className, ...props }: SelectProps) {
  const wired = useFieldControl(props)
  return (
    <select className={cn(controlBase, 'h-10', className)} {...wired}>
      {placeholder !== undefined && <option value="">{placeholder}</option>}
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}
