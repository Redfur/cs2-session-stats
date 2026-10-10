import { Link2 } from 'lucide-react'
import { useId, useState, type ClipboardEvent, type FormEvent, type ReactNode } from 'react'
import { Link } from 'react-router'
import { api, type ImportAddResult } from '../api'
import { formatDateLong } from '../format'
import { Button } from './ui/Button'
import { cx } from './ui/cx'
import { Input, Label } from './ui/Field'

interface MatchLinkFormProps {
  sessionId: string
  // large — под большой зоной пустой сессии (с разделителем «или»), strip — в подвале списка матчей
  variant: 'large' | 'strip'
  // ссылки приняты: страница перезагружает сессию со списком загрузок
  onAdded: () => void
}

// Сообщение под полем: ошибка или подсказка по ссылке, которая не стала новой загрузкой.
interface Note {
  error: boolean
  text: ReactNode
}

function matchNumber(r: ImportAddResult) {
  return r.externalId ?? r.url
}

function sessionLabel(r: ImportAddResult) {
  return r.sessionTitle || (r.sessionDate ? formatDateLong(r.sessionDate) : 'другой сессии')
}

function noteOf(r: ImportAddResult, sessionId: string, many: boolean): Note | null {
  const here = String(r.sessionId) === sessionId
  switch (r.status) {
    case 'error':
      return { error: true, text: many ? `${r.url} — ${r.error}` : r.error }
    case 'exists':
      return {
        error: false,
        text: here ? (
          <>
            Матч {matchNumber(r)} уже есть в сессии —{' '}
            <Link to={`/matches/${r.matchId}`} className="focus-ring">
              матч #{r.ordinal}
            </Link>
            . Повторно не добавлен.
          </>
        ) : (
          <>
            Матч {matchNumber(r)} уже есть в сессии «{sessionLabel(r)}» —{' '}
            <Link to={`/matches/${r.matchId}`} className="focus-ring">
              открыть матч
            </Link>
            . Повторно не добавлен.
          </>
        ),
      }
    case 'downloading':
      return {
        error: false,
        text: here ? (
          `Матч ${matchNumber(r)} уже скачивается в этой сессии.`
        ) : (
          <>
            Матч {matchNumber(r)} уже скачивается в сессии{' '}
            <Link to={`/sessions/${r.sessionId}`} className="focus-ring">
              «{sessionLabel(r)}»
            </Link>
            .
          </>
        ),
      }
    case 'retried':
      return { error: false, text: `Матч ${matchNumber(r)} снова поставлен в очередь скачивания.` }
    default:
      return null
  }
}

// MatchLinkForm — поле «Ссылка на матч»: сервер проверяет матч у платформы и сам скачивает демку.
// В поле можно вставить несколько ссылок через пробел или с новой строки.
export function MatchLinkForm({ sessionId, variant, onAdded }: MatchLinkFormProps) {
  const id = useId()
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)
  const [notes, setNotes] = useState<Note[]>([])
  const errorId = `${id}-err`
  const hasError = notes.some((n) => n.error)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!value.trim() || busy) return
    setBusy(true)
    setNotes([])
    try {
      const results = await api.addImports(sessionId, value)
      const many = results.length > 1
      setNotes(results.flatMap((r) => noteOf(r, sessionId, many) ?? []))
      // в поле остаются только отклонённые ссылки — их можно поправить и отправить снова
      setValue(
        results
          .filter((r) => r.status === 'error')
          .map((r) => r.url)
          .join(' '),
      )
      if (results.some((r) => r.status !== 'error')) onAdded()
    } catch (err) {
      setNotes([{ error: true, text: (err as Error).message }])
    } finally {
      setBusy(false)
    }
  }

  // однострочное поле при вставке выбрасывает переводы строк, и ссылки склеились бы
  function paste(e: ClipboardEvent<HTMLInputElement>) {
    const text = e.clipboardData.getData('text')
    if (!/[\r\n]/.test(text)) return
    e.preventDefault()
    const input = e.currentTarget
    const insert = text.trim().replace(/\s+/g, ' ')
    const start = input.selectionStart ?? value.length
    const end = input.selectionEnd ?? value.length
    setValue(value.slice(0, start) + insert + value.slice(end))
  }

  const errors = notes.filter((n) => n.error)
  const hints = notes.filter((n) => !n.error)

  return (
    <div className="flex flex-col gap-2.5">
      {variant === 'large' && (
        <div className="flex items-center gap-3 text-small font-semibold text-fg-faint before:h-px before:flex-1 before:bg-border after:h-px after:flex-1 after:bg-border">
          или
        </div>
      )}
      <form onSubmit={submit} className="flex flex-col gap-1.5" noValidate>
        <Label htmlFor={id}>{variant === 'large' ? 'Ссылка на матч — демку скачаем сами' : 'Или ссылка на матч — демку скачаем сами'}</Label>
        <div className="flex items-start gap-2">
          <div className="relative flex min-w-0 flex-1 items-center">
            <Link2 className="pointer-events-none absolute left-[11px] size-4 text-fg-muted" aria-hidden />
            <Input
              id={id}
              type="text"
              inputMode="url"
              autoComplete="off"
              spellCheck={false}
              className="pl-9"
              placeholder="https://cs2.fastcup.net/matches/…"
              value={value}
              disabled={busy}
              aria-invalid={hasError || undefined}
              aria-describedby={errors.length > 0 ? errorId : undefined}
              onChange={(e) => setValue(e.target.value)}
              onPaste={paste}
            />
          </div>
          <Button type="submit" variant="secondary" loading={busy} disabled={!value.trim()}>
            {busy ? 'Проверяем…' : 'Добавить'}
          </Button>
        </div>
        {errors.length > 0 && (
          <div id={errorId} role="alert" className="flex flex-col gap-1 text-small text-status-error">
            {errors.map((n, i) => (
              <p key={i} className="m-0 break-words">
                {n.text}
              </p>
            ))}
          </div>
        )}
        {hints.map((n, i) => (
          <p key={i} className={cx('m-0 text-small text-fg-muted break-words')} role="status">
            {n.text}
          </p>
        ))}
        {notes.length === 0 && (
          <p className="m-0 text-small text-fg-muted">Несколько ссылок через пробел или с новой строки станут матчами по времени игры.</p>
        )}
      </form>
    </div>
  )
}
