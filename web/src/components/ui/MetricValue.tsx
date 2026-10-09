import { formatMetric, metricLevel, type Metric } from '../../metrics'
import { cx } from './cx'

const LEVELS = {
  good: 'text-metric-good bg-metric-good/13',
  mid: 'text-metric-mid',
  bad: 'text-metric-bad bg-metric-bad/14',
} as const

// MetricValue — значение показателя, окрашенное по шкале. «Хорошо» и «плохо» отличаются
// не только цветом текста, но и плашкой; уровень дублируется в title.
export function MetricValue({ metric, value, strong = false }: { metric: Metric; value: number; strong?: boolean }) {
  const level = metricLevel(metric, value)
  return (
    <span
      data-level={level}
      title={level === 'good' ? 'хорошо' : level === 'bad' ? 'плохо' : undefined}
      className={cx(
        '-mr-1.5 inline-block min-w-[46px] rounded-[5px] px-1.5 py-0.5 text-right',
        LEVELS[level],
        strong && 'text-[14px] font-extrabold',
      )}
    >
      {formatMetric(metric, value)}
    </span>
  )
}
