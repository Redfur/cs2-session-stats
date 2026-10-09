import { Radio, RadioGroup } from '@headlessui/react'
import { cx } from './cx'

interface SegmentProps<T extends string> {
  value: T
  onChange: (value: T) => void
  options: { value: T; label: string }[]
  label: string
  className?: string
}

// Segment — переключатель режима из нескольких вариантов (радиогруппа).
export function Segment<T extends string>({ value, onChange, options, label, className }: SegmentProps<T>) {
  return (
    <RadioGroup
      value={value}
      onChange={onChange}
      aria-label={label}
      className={cx('flex gap-0.5 rounded-md border border-border-strong bg-bg p-[3px]', className)}
    >
      {options.map((o) => (
        <Radio
          key={o.value}
          value={o.value}
          className={cx(
            'flex h-7 flex-1 cursor-pointer items-center justify-center whitespace-nowrap rounded-sm px-3 text-[13px] font-semibold text-fg-muted transition-colors outline-none',
            'hover:text-fg data-checked:bg-surface-hover data-checked:text-fg data-checked:shadow-[inset_0_0_0_1px_var(--color-border-strong)]',
            'data-focus:shadow-[0_0_0_2px_var(--color-bg),0_0_0_4px_var(--color-accent)]',
          )}
        >
          {o.label}
        </Radio>
      ))}
    </RadioGroup>
  )
}
