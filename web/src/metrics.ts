// Шкала показателей «плохо / средне / хорошо» (спека web-ui) и их форматирование.

export type Metric = 'rating' | 'kd' | 'adr' | 'kast'
export type MetricLevel = 'good' | 'mid' | 'bad'

// [нижняя, верхняя] граница «средне»; сами границы относятся к «средне»
const THRESHOLDS: Record<Metric, [number, number]> = {
  rating: [0.85, 1.15],
  kd: [0.85, 1.15],
  adr: [60, 90],
  kast: [65, 75],
}

// Знаков после запятой при выводе: rating и K/D — 2, ADR — 1, KAST — целые проценты.
const DIGITS: Record<Metric, number> = { rating: 2, kd: 2, adr: 1, kast: 0 }

export function formatMetric(metric: Metric, value: number): string {
  const s = value.toFixed(DIGITS[metric])
  return metric === 'kast' ? `${s}%` : s
}

// Уровень считается по отображаемому округлённому значению, иначе 1.1501 выводится как «1.15»,
// но окрашивается как «хорошо».
export function metricLevel(metric: Metric, value: number): MetricLevel {
  const shown = Number(value.toFixed(DIGITS[metric]))
  const [low, high] = THRESHOLDS[metric]
  if (shown < low) return 'bad'
  if (shown > high) return 'good'
  return 'mid'
}

export function formatPct(value: number): string {
  return `${value.toFixed(0)}%`
}
