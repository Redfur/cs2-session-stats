// Шкала показателей «плохо / средне / хорошо» (спека web-ui) и их форматирование.

export type Metric = 'rating' | 'kd' | 'adr' | 'kast'
export type MetricLevel = 'good' | 'mid' | 'bad'

// [нижняя, верхняя] граница «средне»; сами границы относятся к «средне»
const THRESHOLDS: Record<Metric, [number, number]> = {
  rating: [0.95, 1.25],
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

// Ступень оценки 0–100 (оценка за матч, сессию, «Среднее за 20» и OVR): своя градация,
// не «плохо / средне / хорошо». Нижние границы ступеней 2–5.
const SCORE_TIERS = [50, 65, 75, 90]
export type ScoreTier = 1 | 2 | 3 | 4 | 5

export function scoreTier(score: number): ScoreTier {
  return (SCORE_TIERS.filter((t) => score >= t).length + 1) as ScoreTier
}

// Название ступени оценки — подпись под OVR в профиле.
export function scoreTierName(score: number): string {
  return ['слабо', 'обычно', 'хорошо', 'сильно', 'исключительно'][scoreTier(score) - 1]
}

// Разница в баллах со знаком: «+8», «−5», «±0».
export function formatDelta(d: number): string {
  return d > 0 ? `+${d}` : d < 0 ? `−${-d}` : '±0'
}

// vsUsualText — подсказка сравнения с обычным уровнем: «Лучше обычного на 8 · обычно 74».
export function vsUsualText(delta: number, usual: number): string {
  const what = delta > 0 ? `Лучше обычного на ${delta}` : delta < 0 ? `Хуже обычного на ${-delta}` : 'Как обычно'
  return `${what} · обычно ${usual}`
}

// ovrChangeText — подсказка к изменению OVR за сессию.
export function ovrChangeText(before: number, after: number, delta: number): string {
  return `OVR за сессию: ${before} → ${after} (${formatDelta(delta)})`
}
