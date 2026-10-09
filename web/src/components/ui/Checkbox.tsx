import { Check } from 'lucide-react'
import type { ReactNode } from 'react'
import { cx } from './cx'

interface CheckboxProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  // число справа: матчи игрока или сессии
  count?: ReactNode
  className?: string
  children: ReactNode
}

// Checkbox — нативный чекбокс, скрытый визуально, с собственным квадратом.
export function Checkbox({ checked, onChange, disabled, count, className, children }: CheckboxProps) {
  return (
    <label
      className={cx(
        'group relative flex min-h-8 items-center gap-2.5 font-medium',
        disabled ? 'cursor-not-allowed text-fg-faint' : 'cursor-pointer',
        className,
      )}
    >
      <input
        type="checkbox"
        className="peer pointer-events-none absolute m-0 size-px opacity-0"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span
        className={cx(
          'grid size-[18px] flex-none place-items-center rounded-[5px] border-[1.5px] transition-colors',
          'peer-focus-visible:shadow-[0_0_0_2px_var(--color-bg),0_0_0_4px_var(--color-accent)] peer-disabled:opacity-45',
          checked
            ? 'border-primary bg-primary text-primary-fg'
            : 'border-border-strong bg-surface-2 text-transparent group-hover:border-border-hover',
        )}
        aria-hidden
      >
        <Check className="size-3" strokeWidth={3} />
      </span>
      <span className="flex min-w-0 flex-col leading-[18px]">{children}</span>
      {count != null && <span className="ml-auto text-small text-fg-muted tabular-nums">{count}</span>}
    </label>
  )
}
