import { Description, Dialog, DialogBackdrop, DialogPanel, DialogTitle } from '@headlessui/react'
import { RefreshCw, Trash, X } from 'lucide-react'
import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Alert } from './Alert'
import { Button } from './Button'
import { cx } from './cx'

// Общий каркас: затемнение, панель шириной 400, крестик. Пока идёт запрос, диалог не закрывается.
function Shell({
  open,
  onClose,
  busy,
  role,
  children,
  as,
  onSubmit,
}: {
  open: boolean
  onClose: () => void
  busy: boolean
  role: 'dialog' | 'alertdialog'
  children: ReactNode
  as?: 'form'
  onSubmit?: (e: FormEvent<HTMLFormElement>) => void
}) {
  const close = () => {
    if (!busy) onClose()
  }
  const panel = cx(
    'relative flex w-[400px] max-w-full flex-col gap-4 rounded-xl border border-border-strong bg-surface p-6 shadow-overlay',
  )
  return (
    <Dialog open={open} onClose={close} role={role} className="relative z-50">
      <DialogBackdrop className="fixed inset-0 bg-[rgb(4_5_7/0.78)]" />
      <div className="fixed inset-0 flex items-start justify-center overflow-y-auto px-4 pt-[120px] pb-4 max-md:pt-16">
        <DialogPanel as={as ?? 'div'} className={panel} onSubmit={onSubmit}>
          <Button variant="ghost" size="sm" icon className="absolute top-3.5 right-3.5" aria-label="Закрыть" onClick={close} disabled={busy}>
            <X aria-hidden />
          </Button>
          {children}
        </DialogPanel>
      </div>
    </Dialog>
  )
}

function Title({ children }: { children: ReactNode }) {
  return <DialogTitle className="m-0 pr-8 text-[17px] leading-6 font-bold">{children}</DialogTitle>
}

// useAsyncAction выполняет действие диалога: состояние loading и текст ошибки.
// При новом открытии ошибка сбрасывается.
function useAsyncAction(open: boolean) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const [prevOpen, setPrevOpen] = useState(open)
  if (open !== prevOpen) {
    setPrevOpen(open)
    if (open) setError('')
  }
  async function run(action: () => Promise<void>, onDone: () => void) {
    setBusy(true)
    setError('')
    try {
      await action()
      if (mounted.current) onDone()
    } catch (err) {
      if (mounted.current) setError((err as Error).message)
    } finally {
      if (mounted.current) setBusy(false)
    }
  }
  return { busy, error, run }
}

interface ConfirmDialogProps {
  open: boolean
  onClose: () => void
  title: ReactNode
  // называет объект и последствия
  description: ReactNode
  // повторяет действие глаголом: «Удалить сессию»
  confirmLabel: string
  tone: 'danger' | 'normal'
  // при ошибке диалог остаётся открытым и показывает её текст
  onConfirm: () => Promise<void>
  // заголовок ошибки над кнопками
  errorTitle?: string
}

// ConfirmDialog подтверждает необратимое или массовое действие.
// В опасном диалоге фокус при открытии стоит на «Отмена», поэтому Enter ничего не удаляет.
export function ConfirmDialog({ open, onClose, title, description, confirmLabel, tone, onConfirm, errorTitle }: ConfirmDialogProps) {
  const { busy, error, run } = useAsyncAction(open)
  const danger = tone === 'danger'
  return (
    <Shell open={open} onClose={onClose} busy={busy} role="alertdialog">
      <div
        className={cx(
          'grid size-10 place-items-center rounded-[10px] [&>svg]:size-5',
          danger ? 'bg-metric-bad/14 text-metric-bad' : 'bg-accent/14 text-accent',
        )}
        aria-hidden
      >
        {danger ? <Trash /> : <RefreshCw />}
      </div>
      <Title>{title}</Title>
      <Description as="div" className="text-[14px] leading-[21px] text-fg-muted [&_b]:font-bold [&_b]:text-fg">
        {description}
      </Description>
      {error && (
        <Alert tone="error" title={errorTitle ?? 'Не получилось.'}>
          {error}
        </Alert>
      )}
      <div className="mt-1 flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose} disabled={busy} autoFocus={danger}>
          Отмена
        </Button>
        <Button
          variant={danger ? 'danger' : 'primary'}
          loading={busy}
          autoFocus={!danger}
          leading={danger ? undefined : <RefreshCw aria-hidden />}
          onClick={() => run(onConfirm, onClose)}
        >
          {confirmLabel}
        </Button>
      </div>
    </Shell>
  )
}

interface FormDialogProps {
  open: boolean
  onClose: () => void
  title: ReactNode
  submitLabel: string
  onSubmit: () => Promise<void>
  errorTitle?: string
  children: ReactNode
}

// FormDialog — тот же каркас с формой; Enter в поле отправляет форму.
export function FormDialog({ open, onClose, title, submitLabel, onSubmit, errorTitle, children }: FormDialogProps) {
  const { busy, error, run } = useAsyncAction(open)
  return (
    <Shell
      open={open}
      onClose={onClose}
      busy={busy}
      role="dialog"
      as="form"
      onSubmit={(e) => {
        e.preventDefault()
        run(onSubmit, onClose)
      }}
    >
      <Title>{title}</Title>
      {children}
      {error && (
        <Alert tone="error" title={errorTitle ?? 'Не получилось.'}>
          {error}
        </Alert>
      )}
      <div className="mt-1 flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose} disabled={busy}>
          Отмена
        </Button>
        <Button variant="primary" type="submit" loading={busy}>
          {submitLabel}
        </Button>
      </div>
    </Shell>
  )
}
