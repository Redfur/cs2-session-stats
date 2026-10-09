import type { InputHTMLAttributes, ReactNode, Ref } from 'react'
import { cx } from './cx'

export const inputClass =
  'block h-9 w-full rounded-md border border-border-strong bg-surface-2 px-3 text-[14px] leading-5 font-medium tabular-nums text-fg outline-none transition-[border-color,box-shadow] placeholder:text-placeholder enabled:hover:border-border-hover focus:border-accent focus:shadow-[0_0_0_3px_rgb(232_176_75/0.14)] disabled:cursor-not-allowed disabled:opacity-45'

export function Input({ className, ref, ...rest }: InputHTMLAttributes<HTMLInputElement> & { ref?: Ref<HTMLInputElement> }) {
  return <input ref={ref} className={cx(inputClass, rest.type === 'date' && 'px-2', className)} {...rest} />
}

interface FieldProps {
  label: ReactNode
  htmlFor?: string
  help?: ReactNode
  error?: ReactNode
  className?: string
  children: ReactNode
}

// Field — подпись, поле и подсказка или ошибка под ним.
export function Field({ label, htmlFor, help, error, className, children }: FieldProps) {
  return (
    <div className={cx('flex min-w-0 flex-col gap-1.5', className)}>
      <label htmlFor={htmlFor} className="text-small font-semibold text-fg-muted">
        {label}
      </label>
      {children}
      {error ? (
        <span className="text-small text-status-error" role="alert">
          {error}
        </span>
      ) : (
        help && <span className="text-small text-fg-muted">{help}</span>
      )}
    </div>
  )
}

export function Label({ children, htmlFor, className }: { children: ReactNode; htmlFor?: string; className?: string }) {
  return (
    <label htmlFor={htmlFor} className={cx('text-small font-semibold text-fg-muted', className)}>
      {children}
    </label>
  )
}
