// Расширенная статистика из демок: загрузка блоков, тексты покрытия и форматирование чисел.

import { useEffect, useRef, useState } from 'react'
import type { ExtMetric, MissingMatch, MissingReason } from './api'
import { plural } from './format'

// Пока матчи пересчитываются, данные расширенной статистики запрашиваются снова через этот интервал.
const REPARSE_POLL_MS = 4000

export type Load<T> = { data: T | null; error: string; reload: () => void }

// useExtLoad загружает блок расширенной статистики отдельно от страницы. key описывает запрос;
// пока в выборке есть пересчитываемые матчи, данные обновляются сами.
export function useExtLoad<T extends object>(fetch: () => Promise<T>, key: string): Load<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  const fetchRef = useRef(fetch)
  useEffect(() => {
    fetchRef.current = fetch
  })
  useEffect(() => {
    let stale = false
    fetchRef.current().then(
      (d) => {
        if (stale) return
        setData(d)
        setError('')
      },
      (e: Error) => !stale && setError(e.message),
    )
    return () => {
      stale = true
    }
  }, [key, attempt])
  const reparsing = ((data as { reparsing?: unknown[] } | null)?.reparsing?.length ?? 0) > 0
  useEffect(() => {
    if (!reparsing) return
    const t = setTimeout(() => setAttempt((a) => a + 1), REPARSE_POLL_MS)
    return () => clearTimeout(t)
  }, [reparsing, data])
  return { data, error, reload: () => setAttempt((a) => a + 1) }
}

// ── Тексты покрытия ──

export const byMaps = (n: number) => `по ${plural(n, ['карте', 'картам', 'картам'])}`

function missingText(m: MissingMatch, single: boolean): string {
  const map = m.map || 'карты'
  switch (m.reason) {
    case 'old':
      return single ? 'Матч обработан старой версией' : `матч #${m.ordinal} (${map}) обработан старой версией`
    case 'no_damage':
      return single ? 'В демке нет событий урона' : `в демке ${map} нет событий урона`
    case 'no_flash':
      return single ? 'В демке нет событий ослепления' : `в демке ${map} нет событий ослепления`
  }
}

const REASON_SHORT: Record<MissingReason, string> = {
  old: 'Матч обработан старой версией — пересчитайте его',
  no_damage: 'В демке нет событий урона',
  no_flash: 'В демке нет событий ослепления',
}

// missingTip — подсказка: каких карт нет в значении и почему.
export function missingTip(missing: MissingMatch[]): string {
  if (missing.length === 0) return ''
  return `Нет данных за ${plural(missing.length, ['карту', 'карты', 'карт'])}: ${missing.map((m) => missingText(m, false)).join('; ')}`
}

// notComputedReason — почему у значения нет ни одного покрытого матча.
export function notComputedReason(m: ExtMetric): string {
  const reasons = new Set(m.missing.map((x) => x.reason))
  if (reasons.size === 1) return REASON_SHORT[m.missing[0].reason] + ' — не посчитано'
  return missingTip(m.missing) || 'Не посчитано'
}

// coverageChip — текст чипа покрытия, если метрика посчитана не по всем картам.
export function coverageNote(m: ExtMetric): string | null {
  if (m.total === 0 || m.covered === m.total) return null
  return `по ${m.covered} из ${m.total} карт`
}

// ── Форматирование ──

const NBSP = ' '

export const fmt1 = (v: number) => v.toFixed(1)
export const fmtInt = (v: number) => String(Math.round(v)).replace(/\B(?=(\d{3})+(?!\d))/g, NBSP)
export const fmtPct = (v: number) => `${Math.round(v)}%`
export const fmtSigned = (v: number) => (v > 0 ? '+' : v < 0 ? '−' : '') + Math.abs(v).toFixed(1)
export const sharePct = (part: number, whole: number) => (whole > 0 ? Math.round((100 * part) / whole) : null)
