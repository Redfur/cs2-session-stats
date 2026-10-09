import { Ban, Check, Copy, LoaderCircle, TriangleAlert, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { formatBytes, formatBytesOf, formatEta } from '../../format'
import { isPending, type UploadItem } from '../../upload'
import { Badge } from './Badge'
import { Button } from './Button'
import { cx } from './cx'

interface UploadListProps {
  items: UploadItem[]
  // номер, который получит первый ещё не принятый файл
  nextNumber: number
  sessionId: number
  // номер матча в сессии по id; undefined, пока сессия не перезагружена
  ordinalOf: (matchId: number) => number | undefined
  onCancel: (id: number) => void
  onCancelAll: () => void
  onClear: () => void
}

function Bar({ value, done, label }: { value: number; done?: boolean; label: string }) {
  return (
    <div
      className="mt-2 h-1 overflow-hidden rounded-xs bg-surface-hover"
      role="progressbar"
      aria-valuenow={Math.round(value)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
    >
      <span className={cx('block h-full rounded-xs', done ? 'bg-status-ok' : 'bg-status-progress')} style={{ width: `${value}%` }} />
    </div>
  )
}

// UploadList — очередь загрузки: номер будущего матча, прогресс по каждому файлу и итог.
export function UploadList({ items, nextNumber, sessionId, ordinalOf, onCancel, onCancelAll, onClear }: UploadListProps) {
  if (items.length === 0) return null
  const active = items.some(isPending)
  const counted = items.filter((it) => it.state !== 'cancelled')
  const totalBytes = counted.reduce((s, it) => s + it.file.size, 0)
  const sentBytes = counted.reduce((s, it) => s + (isPending(it) ? it.loaded : it.file.size), 0)
  const current = items.findIndex((it) => it.state === 'sending')

  const count = (state: UploadItem['state']) => items.filter((it) => it.state === state).length
  const summary = [
    count('accepted') && `принято ${count('accepted')}`,
    count('duplicate') && `уже было ${count('duplicate')}`,
    count('error') && `ошибка ${count('error')}`,
    count('cancelled') && `отменено ${count('cancelled')}`,
  ]
    .filter(Boolean)
    .join(' · ')

  let pendingIndex = 0
  return (
    <div className="border-t border-border" aria-live="polite">
      <div className="flex min-h-12 flex-wrap items-center justify-between gap-3 px-4 py-2.5 md:px-5">
        <div className="flex flex-wrap items-baseline gap-3">
          <h3 className="m-0 text-sub">{active ? 'Загрузка' : 'Загрузка завершена'}</h3>
          <span className="text-small text-fg-muted tabular-nums">
            {active ? `${current + 1} из ${items.length} · ${formatBytesOf(sentBytes, totalBytes)}` : summary}
          </span>
        </div>
        {active ? (
          <Button variant="ghost" size="sm" onClick={onCancelAll}>
            Отменить всё
          </Button>
        ) : (
          <Button variant="ghost" size="sm" onClick={onClear}>
            Скрыть
          </Button>
        )}
      </div>
      {items.map((it) => {
        const pending = isPending(it)
        const number = pending ? nextNumber + pendingIndex++ : undefined
        const ordinal = it.state === 'accepted' && it.result?.matchId ? ordinalOf(it.result.matchId) : undefined
        const size = formatBytes(it.file.size)
        let meta: ReactNode
        let right: ReactNode = null
        let bar: ReactNode = null
        if (it.state === 'queued') {
          bar = <Bar value={0} label={`Загрузка файла ${it.file.name}`} />
          meta = `${size} · ждёт очереди`
          right = (
            <Button variant="ghost" size="sm" icon aria-label={`Убрать файл ${it.file.name} из очереди`} onClick={() => onCancel(it.id)}>
              <X aria-hidden />
            </Button>
          )
        } else if (it.state === 'sending' && it.loaded < it.total) {
          const pct = it.total ? (it.loaded / it.total) * 100 : 0
          bar = <Bar value={pct} label={`Загрузка файла ${it.file.name}`} />
          meta = (
            <>
              <span className="font-bold text-status-progress">{Math.floor(pct)}%</span>
              {formatBytesOf(it.loaded, it.total)}
              {it.eta !== undefined && ` · ${formatEta(it.eta)}`}
            </>
          )
          right = (
            <Button variant="ghost" size="sm" icon aria-label={`Отменить загрузку файла ${it.file.name}`} onClick={() => onCancel(it.id)}>
              <X aria-hidden />
            </Button>
          )
        } else if (it.state === 'sending') {
          // файл отправлен целиком, сервер проверяет и сохраняет его — отменить уже нельзя
          bar = <Bar value={100} done label={`Загрузка файла ${it.file.name}`} />
          meta = `${size} · загружен, ждём ответа сервера`
          right = (
            <Badge tone="progress" icon={<LoaderCircle className="animate-spin" aria-hidden />}>
              Проверка
            </Badge>
          )
        } else if (it.state === 'accepted') {
          meta = (
            <>
              {size} ·{' '}
              <Link to={`/matches/${it.result?.matchId}`} className="focus-ring">
                открыть матч{ordinal ? ` #${ordinal}` : ''}
              </Link>
            </>
          )
          right = (
            <Badge tone="ok" icon={<Check aria-hidden />}>
              Принят
            </Badge>
          )
        } else if (it.state === 'duplicate') {
          const here = it.result?.sessionId === sessionId
          meta = (
            <>
              Эта демка уже есть{here ? ' в сессии' : ' в другой сессии'} —{' '}
              <Link to={`/matches/${it.result?.matchId}`} className="focus-ring">
                открыть матч
              </Link>
              . Повторно не добавлена.
            </>
          )
          right = (
            <Badge tone="neutral" icon={<Copy aria-hidden />}>
              Уже загружен
            </Badge>
          )
        } else if (it.state === 'error') {
          meta = <span className="text-status-error">{it.error}</span>
          right = (
            <Badge tone="error" icon={<TriangleAlert aria-hidden />}>
              Ошибка
            </Badge>
          )
        } else {
          meta = 'Отменён, файл не сохранён'
          right = (
            <Badge tone="neutral" icon={<Ban aria-hidden />}>
              Отменён
            </Badge>
          )
        }
        const ord = number ?? ordinal
        return (
          <div
            key={it.id}
            className="grid grid-cols-[32px_minmax(0,1fr)_auto] items-center gap-x-3.5 gap-y-1 border-t border-border px-4 py-3.5 md:px-5"
          >
            <span className={cx('grid h-6 w-8 place-items-center rounded-sm bg-surface-2 text-small font-bold tabular-nums', ord ? 'text-fg-muted' : 'text-fg-faint')}>
              {ord ? `#${ord}` : '—'}
            </span>
            <div className="min-w-0">
              <div className="overflow-hidden font-mono text-[13px] text-ellipsis whitespace-nowrap" title={it.file.name}>
                {it.file.name}
              </div>
              {bar}
              <div className="mt-1 flex flex-wrap items-center gap-x-2 text-small text-fg-muted tabular-nums">{meta}</div>
            </div>
            {right}
          </div>
        )
      })}
    </div>
  )
}
