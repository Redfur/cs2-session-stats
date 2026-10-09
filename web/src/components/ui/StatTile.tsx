import { ArrowDown, ArrowUp } from 'lucide-react'
import type { ReactNode } from 'react'
import { metricLevel, type Metric } from '../../metrics'
import { cx } from './cx'

interface StatTileProps {
  label: string
  value: ReactNode
  sub?: ReactNode
  // окрашивание по шкале показателей
  metric?: { metric: Metric; value: number }
}

const LEVEL_TEXT = { good: 'text-metric-good', mid: 'text-fg', bad: 'text-metric-bad' } as const
const LEVEL_NAME = { good: 'хорошо', mid: 'средне', bad: 'плохо' } as const

// StatTile — крупный показатель профиля.
export function StatTile({ label, value, sub, metric }: StatTileProps) {
  const level = metric ? metricLevel(metric.metric, metric.value) : null
  return (
    <div className="flex min-w-0 flex-col gap-1.5 rounded-lg border border-border bg-surface px-[18px] py-4" data-level={level ?? undefined}>
      <span className="text-over font-bold uppercase text-fg-muted">{label}</span>
      <span
        className={cx('text-[26px] leading-[30px] font-extrabold tracking-[-0.02em] tabular-nums md:text-stat', level && LEVEL_TEXT[level])}
      >
        {value}
        {/* уровень отличается не только цветом: стрелка и текст для скринридера */}
        {level === 'good' && <ArrowUp className="ml-1 inline size-5 align-[-2px]" strokeWidth={2.5} aria-hidden />}
        {level === 'bad' && <ArrowDown className="ml-1 inline size-5 align-[-2px]" strokeWidth={2.5} aria-hidden />}
        {level && level !== 'mid' && <span className="sr-only"> ({LEVEL_NAME[level]})</span>}
      </span>
      {sub && <span className="text-small text-fg-muted tabular-nums">{sub}</span>}
    </div>
  )
}
