import { CalendarPlus, ChevronRight, Plus, Search, TriangleAlert } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { api, todayISO, type SessionSummary } from '../api'
import { PageError, PageLoading } from '../components/PageState'
import { Badge } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { FormDialog } from '../components/ui/Dialog'
import { EmptyState } from '../components/ui/EmptyState'
import { Field, Input } from '../components/ui/Field'
import { Page } from '../components/ui/Layout'
import { PageHeader } from '../components/ui/PageHeader'
import { formatDateLong, formatDateShort, formatMonth, formatWeekday, plural } from '../format'
import { metricLevel } from '../metrics'

// Строка поиска совпадает с названием или датой в любом из видов: 2026-10-09, 09.10.2026, 9 октября 2026.
function matches(s: SessionSummary, q: string): boolean {
  const hay = [s.title, s.date, formatDateShort(s.date), formatDateLong(s.date)].join('\n').toLowerCase()
  return hay.includes(q)
}

export function SessionsPage() {
  const [sessions, setSessions] = useState<SessionSummary[] | null>(null)
  const [error, setError] = useState('')
  const [q, setQ] = useState('')
  const [creating, setCreating] = useState(false)

  const load = useCallback(() => {
    api.listSessions().then(
      (list) => {
        setSessions(list)
        setError('')
      },
      (e: Error) => setError(e.message),
    )
  }, [])
  useEffect(load, [load])

  const groups = useMemo(() => {
    const query = q.trim().toLowerCase()
    const out: { month: string; items: SessionSummary[] }[] = []
    for (const s of sessions ?? []) {
      if (query && !matches(s, query)) continue
      const month = formatMonth(s.date)
      const last = out[out.length - 1]
      if (last?.month === month) last.items.push(s)
      else out.push({ month, items: [s] })
    }
    return out
  }, [sessions, q])

  if (error && !sessions) return <PageError title="Не удалось загрузить сессии" error={error} onRetry={load} back={{ to: '/players', label: 'К игрокам' }} />
  if (!sessions) return <PageLoading />

  const totalMatches = sessions.reduce((n, s) => n + s.matchCount, 0)
  const newButton = (
    <Button variant="primary" leading={<Plus aria-hidden />} onClick={() => setCreating(true)}>
      Новая сессия
    </Button>
  )

  return (
    <Page>
      <PageHeader
        title="Сессии"
        meta={
          sessions.length > 0 && (
            <span className="tabular-nums">
              {plural(sessions.length, ['вечер', 'вечера', 'вечеров'])} · {plural(totalMatches, ['матч', 'матча', 'матчей'])}
            </span>
          )
        }
        actions={
          sessions.length > 0 && (
            <>
              <label className="relative flex w-full items-center sm:w-[280px]">
                <Search className="pointer-events-none absolute left-[11px] size-4 text-fg-muted" aria-hidden />
                <span className="sr-only">Найти вечер</span>
                <Input type="search" className="pl-9" placeholder="Название или дата" value={q} onChange={(e) => setQ(e.target.value)} />
              </label>
              {newButton}
            </>
          )
        }
      />

      {sessions.length === 0 && (
        <Card as="div">
          <EmptyState
            icon={<CalendarPlus />}
            title="Сессий пока нет"
            text="Сессия — это вечер игр. Создайте её и загрузите демки матчей."
            actions={newButton}
          />
        </Card>
      )}

      {groups.map((g) => (
        <section key={g.month} className="flex flex-col gap-2.5" aria-label={g.month}>
          <h2 className="m-0 text-over font-bold uppercase text-fg-muted">{g.month}</h2>
          <Card as="div">
            {g.items.map((s) => (
              <SessionRow key={s.id} s={s} />
            ))}
          </Card>
        </section>
      ))}

      {sessions.length > 0 && groups.length === 0 && (
        <Card as="div">
          <EmptyState
            icon={<Search />}
            title="Ничего не нашлось"
            text={<>Нет вечеров, где в названии или дате есть «{q.trim()}».</>}
            actions={
              <Button variant="ghost" size="sm" onClick={() => setQ('')}>
                Сбросить поиск
              </Button>
            }
          />
        </Card>
      )}

      <NewSessionDialog open={creating} onClose={() => setCreating(false)} />
    </Page>
  )
}

function SessionRow({ s }: { s: SessionSummary }) {
  const errors = s.failedCount > 0 && plural(s.failedCount, ['ошибка', 'ошибки', 'ошибок'])
  return (
    <Link
      to={`/sessions/${s.id}`}
      className="focus-ring grid grid-cols-[44px_minmax(0,1fr)_16px] items-center gap-3 rounded-none border-t border-border px-4 py-3 text-fg first:border-t-0 hover:bg-surface-hover hover:text-fg hover:no-underline md:grid-cols-[52px_minmax(0,1fr)_auto_20px] md:gap-[18px] md:px-5 md:py-3.5"
    >
      <span className="flex size-11 flex-col items-center justify-center rounded-[10px] border border-border bg-surface-2 md:size-[52px]">
        <b className="text-[17px] leading-[18px] font-extrabold tabular-nums md:text-[20px] md:leading-[22px]">{s.date.slice(8)}</b>
        <span className="text-[11px] leading-[14px] font-bold uppercase text-fg-muted">{formatWeekday(s.date)}</span>
      </span>
      <span className="min-w-0">
        <span className="block overflow-hidden text-[15px] leading-5 font-bold text-ellipsis whitespace-nowrap">
          {s.title || formatDateLong(s.date)}
        </span>
        <span className="mt-1 flex flex-wrap gap-x-3.5 gap-y-1 text-small text-fg-muted">
          <span>{s.title ? formatDateLong(s.date) : 'без названия'}</span>
          {s.matchCount > 0 && <span className="tabular-nums">{plural(s.matchCount, ['матч', 'матча', 'матчей'])}</span>}
          {s.maps.length > 0 && <span className="text-fg-faint">{s.maps.join(', ')}</span>}
          {/* на узком экране правая колонка скрыта, ошибки показываются в мета-строке */}
          {errors && <span className="text-status-error md:hidden">{errors}</span>}
        </span>
      </span>
      <span className="hidden items-center gap-5 md:flex">
        {errors && (
          <Badge tone="error" icon={<TriangleAlert aria-hidden />}>
            {errors}
          </Badge>
        )}
        {s.matchCount === 0 && <Badge tone="neutral">нет матчей</Badge>}
        <span className="flex min-w-[110px] flex-col items-end gap-0.5">
          {s.best ? (
            <>
              <span className="text-over font-bold uppercase text-fg-muted">Лучший</span>
              <span>
                <b className="font-bold">{s.best.name}</b>{' '}
                <span
                  className={
                    'font-extrabold tabular-nums ' +
                    { good: 'text-metric-good', mid: 'text-fg', bad: 'text-metric-bad' }[metricLevel('rating', s.best.rating)]
                  }
                >
                  {s.best.rating.toFixed(2)}
                </span>
              </span>
            </>
          ) : (
            <span className="text-small text-fg-muted">загрузите демки</span>
          )}
        </span>
      </span>
      <ChevronRight className="size-4 text-fg-faint" aria-hidden />
    </Link>
  )
}

function NewSessionDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [date, setDate] = useState(todayISO)
  const [title, setTitle] = useState('')
  const navigate = useNavigate()
  const [prevOpen, setPrevOpen] = useState(open)
  if (open !== prevOpen) {
    setPrevOpen(open)
    if (open) {
      setDate(todayISO())
      setTitle('')
    }
  }
  return (
    <FormDialog
      open={open}
      onClose={onClose}
      title="Новая сессия"
      submitLabel="Создать"
      errorTitle="Не удалось создать сессию."
      onSubmit={async () => {
        const s = await api.createSession(date, title)
        navigate(`/sessions/${s.id}`)
      }}
    >
      <Field label="Дата" htmlFor="ns-date">
        <Input id="ns-date" type="date" required value={date} onChange={(e) => setDate(e.target.value)} />
      </Field>
      <Field label="Название" htmlFor="ns-name" help="Если пусто — покажем дату">
        <Input
          id="ns-name"
          type="text"
          placeholder="необязательно"
          maxLength={200}
          autoFocus
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </Field>
    </FormDialog>
  )
}
