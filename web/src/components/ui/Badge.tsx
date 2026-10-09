import { Check, Clock, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import type { Match } from '../../api'
import { cx } from './cx'

export type BadgeTone = 'ok' | 'progress' | 'error' | 'neutral'

const TONES: Record<BadgeTone, string> = {
  ok: 'text-status-ok bg-status-ok/13',
  progress: 'text-status-progress bg-status-progress/14',
  error: 'text-status-error bg-status-error/12',
  neutral: 'text-status-neutral bg-status-neutral/13',
}

export function Badge({ tone, icon, children }: { tone: BadgeTone; icon?: ReactNode; children: ReactNode }) {
  return (
    <span
      className={cx(
        'inline-flex h-6 items-center gap-1.5 whitespace-nowrap rounded-full pr-[9px] pl-[7px] text-small font-semibold [&>svg]:size-3.5',
        !icon && 'pl-[9px]',
        TONES[tone],
      )}
    >
      {icon}
      {children}
    </span>
  )
}

// Статус обработки матча; у матча с прежним результатом pending/parsing — это пересчёт.
function matchBadge(m: Pick<Match, 'status' | 'hasResult'>): { tone: BadgeTone; icon: ReactNode; label: string } {
  switch (m.status) {
    case 'pending':
      return m.hasResult
        ? { tone: 'neutral', icon: <RefreshCw aria-hidden />, label: 'Пересчёт в очереди' }
        : { tone: 'neutral', icon: <Clock aria-hidden />, label: 'В очереди' }
    case 'parsing':
      return m.hasResult
        ? { tone: 'progress', icon: <RefreshCw className="animate-spin" aria-hidden />, label: 'Пересчитывается' }
        : { tone: 'progress', icon: <LoaderCircle className="animate-spin" aria-hidden />, label: 'Обрабатывается' }
    case 'done':
      return { tone: 'ok', icon: <Check aria-hidden />, label: 'Готово' }
    case 'failed':
      return { tone: 'error', icon: <TriangleAlert aria-hidden />, label: 'Ошибка' }
  }
}

// StatusBadge — бейдж статуса матча; у ошибки под бейджем виден её текст.
export function StatusBadge({ match, showError = true }: { match: Pick<Match, 'status' | 'hasResult' | 'error'>; showError?: boolean }) {
  const b = matchBadge(match)
  return (
    <span className="inline-flex flex-col items-start">
      <Badge tone={b.tone} icon={b.icon}>
        {b.label}
      </Badge>
      {showError && match.status === 'failed' && match.error && (
        <span className="mt-1.5 block whitespace-normal text-small text-status-error">{match.error}</span>
      )}
    </span>
  )
}

const OUTCOMES = {
  win: { label: 'Победа', cls: 'text-metric-good bg-metric-good/13' },
  loss: { label: 'Поражение', cls: 'text-metric-bad bg-metric-bad/14' },
  draw: { label: 'Ничья', cls: 'text-status-neutral bg-status-neutral/13' },
} as const

export function OutcomeBadge({ result }: { result: 'win' | 'loss' | 'draw' }) {
  const o = OUTCOMES[result]
  return (
    <span className={cx('inline-flex h-[22px] items-center whitespace-nowrap rounded-[5px] px-2 text-small font-bold', o.cls)}>
      {o.label}
    </span>
  )
}
