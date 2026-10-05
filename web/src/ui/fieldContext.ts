import { createContext, useContext } from 'react'

/** What a Field hands to the control inside it, so label, hint and error are wired by id. */
export interface FieldControlProps {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: true
  required?: boolean
}

export const FieldContext = createContext<FieldControlProps | null>(null)

/** Merges the surrounding Field's wiring into a control's own props; explicit props win. */
export function useFieldControl<P extends { id?: string; invalid?: boolean; required?: boolean; 'aria-describedby'?: string }>(
  props: P,
): Omit<P, 'invalid'> & { 'aria-invalid'?: true } {
  const field = useContext(FieldContext)
  const { invalid, ...rest } = props
  return {
    ...rest,
    id: props.id ?? field?.id,
    required: props.required ?? field?.required,
    'aria-describedby': props['aria-describedby'] ?? field?.['aria-describedby'],
    'aria-invalid': invalid || field?.['aria-invalid'] ? true : undefined,
  }
}
