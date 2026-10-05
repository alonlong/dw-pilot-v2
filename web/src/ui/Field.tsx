import { useId, useMemo, type ReactNode } from 'react'
import * as LabelPrimitive from '@radix-ui/react-label'
import { cn } from './cn'
import { FieldContext, type FieldControlProps } from './fieldContext'

export interface FieldProps {
  label: string
  /** Error text shown under the control; also marks the control aria-invalid. */
  error?: string
  /** Helper text shown under the control when there is no error. */
  hint?: string
  required?: boolean
  className?: string
  /** Exactly one Input, Textarea or Select. */
  children: ReactNode
}

/** Label + control + hint/error, wired together by id so screen readers read all three. */
export function Field({ label, error, hint, required, className, children }: FieldProps) {
  const id = useId()
  const messageId = `${id}-message`
  const message = error ?? hint
  const ctx = useMemo<FieldControlProps>(
    () => ({
      id,
      required,
      'aria-describedby': message ? messageId : undefined,
      'aria-invalid': error ? true : undefined,
    }),
    [id, required, message, messageId, error],
  )
  return (
    <div className={cn('flex flex-col gap-1', className)}>
      <LabelPrimitive.Root htmlFor={id} className="text-sm font-medium text-fg">
        {label}
        {required && (
          <span className="text-danger" aria-hidden="true">
            {' '}*
          </span>
        )}
      </LabelPrimitive.Root>
      <FieldContext.Provider value={ctx}>{children}</FieldContext.Provider>
      {message && (
        <p id={messageId} className={cn('text-xs', error ? 'text-danger' : 'text-muted')}>
          {message}
        </p>
      )}
    </div>
  )
}
