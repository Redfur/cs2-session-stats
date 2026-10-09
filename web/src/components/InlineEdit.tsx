import { Check, X } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { Button } from './ui/Button'
import { cx } from './ui/cx'
import { Input } from './ui/Field'

interface InlineEditProps {
  // title — поле размером с заголовок страницы, date — выбор даты в мета-строке
  kind: 'title' | 'date'
  value: string
  label: string
  placeholder?: string
  icon?: ReactNode
  onSave: (value: string) => Promise<void>
  onDone: () => void
}

// InlineEdit — правка одного поля на месте: Enter или ✓ сохраняет, Esc или ✕ отменяет.
// При ошибке поле остаётся открытым и показывает текст ошибки под собой.
export function InlineEdit({ kind, value: initial, label, placeholder, icon, onSave, onDone }: InlineEditProps) {
  const [value, setValue] = useState(initial)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const title = kind === 'title'
  const id = `edit-${kind}`

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      await onSave(value)
      onDone()
    } catch (err) {
      setError((err as Error).message)
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className={cx('flex flex-col gap-1.5', title && 'w-full')}>
      <div className="flex items-center gap-1.5">
        {icon}
        <label htmlFor={id} className="sr-only">
          {label}
        </label>
        <Input
          id={id}
          type={title ? 'text' : 'date'}
          required={!title}
          maxLength={title ? 200 : undefined}
          autoFocus
          value={value}
          placeholder={placeholder}
          disabled={saving}
          aria-invalid={error ? true : undefined}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.preventDefault()
              onDone()
            }
          }}
          className={cx(
            title
              ? 'h-[42px] w-[440px] max-w-full -ml-[11px] px-2.5 text-[24px] leading-[30px] font-bold tracking-[-0.01em] md:text-page'
              : 'h-[30px] w-auto text-fg',
          )}
        />
        <Button type="submit" variant="primary" icon size={title ? 'md' : 'sm'} loading={saving} aria-label={`Сохранить: ${label.toLowerCase()}`}>
          <Check aria-hidden />
        </Button>
        <Button variant="secondary" icon size={title ? 'md' : 'sm'} disabled={saving} aria-label="Отменить" onClick={onDone}>
          <X aria-hidden />
        </Button>
      </div>
      {error && (
        <span className="text-small text-status-error" role="alert">
          {error}
        </span>
      )}
    </form>
  )
}
