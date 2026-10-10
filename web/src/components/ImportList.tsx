import { Check, Clock, Copy, Download, RotateCw, TriangleAlert, X } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { api, isImportActive, type Import, type UploadResult } from '../api'
import { formatBytes, formatBytesOf } from '../format'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { ProgressBar } from './ui/ProgressBar'

interface ImportListProps {
  imports: Import[]
  // номер матча в сессии по id; undefined — матч в другой сессии или ещё не загружен
  ordinalOf: (matchId: number) => number | undefined
  // после действия страница перезагружает сессию
  onChanged: () => void
  onError: (title: string, text: string) => void
}

const SERVER_NOTE = 'качаем на сервере, вкладку можно закрыть'

const platformTitle: Record<string, string> = { fastcup: 'FastCup', cybershoke: 'Cybershoke' }

function resultLine(r: UploadResult, i: number, many: boolean, ordinalOf: ImportListProps['ordinalOf']): ReactNode {
  const map = many ? `Карта ${i + 1}` : 'Демка'
  const ordinal = r.matchId ? ordinalOf(r.matchId) : undefined
  if (r.status === 'accepted')
    return (
      <>
        {map} →{' '}
        <Link to={`/matches/${r.matchId}`} className="focus-ring">
          матч{ordinal ? ` #${ordinal}` : ''}
        </Link>
      </>
    )
  if (r.status === 'duplicate')
    return (
      <>
        {map} уже загружена —{' '}
        <Link to={`/matches/${r.matchId}`} className="focus-ring">
          {ordinal ? `матч #${ordinal}` : 'открыть матч'}
        </Link>
      </>
    )
  return (
    <span className="text-status-error">
      {map}: {r.error}
    </span>
  )
}

// ImportList — загрузки по ссылкам на матчи: сервер скачивает демки сам, страница показывает прогресс и итог.
export function ImportList({ imports, ordinalOf, onChanged, onError }: ImportListProps) {
  const [busy, setBusy] = useState<number | null>(null)
  if (imports.length === 0) return null
  const active = imports.filter(isImportActive).length

  async function act(x: Import, kind: 'retry' | 'delete') {
    setBusy(x.id)
    try {
      if (kind === 'retry') await api.retryImport(x.id)
      else await api.deleteImport(x.id)
    } catch (err) {
      onError(kind === 'retry' ? 'Не удалось повторить скачивание.' : 'Не удалось убрать загрузку.', (err as Error).message)
    } finally {
      setBusy(null)
      onChanged()
    }
  }

  return (
    <div className="border-t border-border" aria-live="polite">
      <div className="flex min-h-12 flex-wrap items-baseline gap-3 px-4 py-2.5 md:px-5">
        <h3 className="m-0 text-sub">Загрузки по ссылкам</h3>
        <span className="text-small text-fg-muted tabular-nums">{active > 0 ? `в работе: ${active}` : 'скачивание закончено'}</span>
      </div>
      {imports.map((x) => {
        const results = x.results ?? []
        const name = `${platformTitle[x.platform] ?? x.platform} ${x.externalId}`
        let bar: ReactNode = null
        let meta: ReactNode = null
        let badge: ReactNode
        const actions: ReactNode[] = []
        const remove = (label: string) => (
          <Button key="remove" variant="ghost" size="sm" icon aria-label={`${label}: ${name}`} title={label} disabled={busy === x.id} onClick={() => act(x, 'delete')}>
            <X aria-hidden />
          </Button>
        )

        if (x.status === 'queued') {
          meta = `в очереди · ${SERVER_NOTE}`
          badge = (
            <Badge tone="neutral" icon={<Clock aria-hidden />}>
              В очереди
            </Badge>
          )
          actions.push(remove('Отменить'))
        } else if (x.status === 'downloading') {
          const total = x.bytesTotal
          const pct = total ? Math.min(100, (x.bytesDone / total) * 100) : undefined
          bar = <ProgressBar value={pct ?? 0} label={`Скачивание демки матча ${name}`} />
          meta = (
            <>
              {pct !== undefined && <span className="font-bold text-status-progress">{Math.floor(pct)}%</span>}
              {total ? formatBytesOf(x.bytesDone, total) : formatBytes(x.bytesDone)} · {SERVER_NOTE}
            </>
          )
          badge = (
            <Badge tone="progress" icon={<Download aria-hidden />}>
              Скачивается
            </Badge>
          )
          actions.push(remove('Отменить'))
        } else if (x.status === 'failed') {
          meta = <span className="text-status-error">{x.error}</span>
          badge = (
            <Badge tone="error" icon={<TriangleAlert aria-hidden />}>
              Ошибка
            </Badge>
          )
          actions.push(
            <Button key="retry" variant="secondary" size="sm" leading={<RotateCw aria-hidden />} loading={busy === x.id} onClick={() => act(x, 'retry')}>
              Повторить
            </Button>,
            remove('Убрать'),
          )
        } else {
          const dup = results.every((r) => r.status === 'duplicate')
          badge = dup ? (
            <Badge tone="neutral" icon={<Copy aria-hidden />}>
              Уже загружен
            </Badge>
          ) : (
            <Badge tone="ok" icon={<Check aria-hidden />}>
              Готово
            </Badge>
          )
          actions.push(remove('Убрать'))
        }

        const many = results.length > 1
        return (
          <div key={x.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3.5 gap-y-1 border-t border-border px-4 py-3.5 md:px-5">
            <div className="min-w-0">
              <a
                href={x.url}
                target="_blank"
                rel="noreferrer"
                title={x.url}
                className="focus-ring block overflow-hidden font-mono text-[13px] text-ellipsis whitespace-nowrap text-fg no-underline hover:text-accent"
              >
                {x.url}
              </a>
              {bar}
              {meta && <div className="mt-1 flex flex-wrap items-center gap-x-2 text-small text-fg-muted tabular-nums">{meta}</div>}
              {results.length > 0 && (
                <ul className="m-0 mt-1 flex list-none flex-col gap-0.5 p-0 text-small text-fg-muted">
                  {results.map((r, i) => (
                    <li key={i}>{resultLine(r, i, many, ordinalOf)}</li>
                  ))}
                </ul>
              )}
            </div>
            <div className="flex items-center gap-2">
              {actions.length > 1 && actions[0]}
              {badge}
              {actions.length > 1 ? actions.slice(1) : actions}
            </div>
          </div>
        )
      })}
    </div>
  )
}
