// Доля и ступень личного счёта пары для матрицы дуэлей (спека personal-duels).

export type DuelLevel = 1 | 2 | 3 | 4 | 5

// Ступени доли: до 25%, 26–44%, 45–55%, 56–74%, от 75%.
export const DUEL_LEVELS: { level: DuelLevel; label: string; name: string }[] = [
  { level: 1, label: '≤25%', name: 'до 25 %' },
  { level: 2, label: '26–44', name: '26–44 %' },
  { level: 3, label: '45–55', name: '45–55 %' },
  { level: 4, label: '56–74', name: '56–74 %' },
  { level: 5, label: '≥75%', name: 'от 75 %' },
]

// Крайние ступени — только когда в паре столько убийств, иначе 0:1 выглядел бы разгромом.
const EXTREME_MIN_KILLS = 4

// duelPct — доля kills / (kills + deaths) в целых процентах. При неравном счёте 50% не выводится:
// 101:100 — это 51%, а не ничья.
export function duelPct(kills: number, deaths: number): number {
  let p = Math.round((100 * kills) / (kills + deaths))
  if (kills > deaths && p <= 50) p = 51
  if (kills < deaths && p >= 50) p = 49
  return p
}

// duelLevel — ступень доли по выводимому проценту; при 0:0 ступени нет.
export function duelLevel(kills: number, deaths: number): DuelLevel | null {
  if (kills + deaths === 0) return null
  const p = duelPct(kills, deaths)
  let level: DuelLevel = p <= 25 ? 1 : p < 45 ? 2 : p <= 55 ? 3 : p < 75 ? 4 : 5
  if (kills + deaths < EXTREME_MIN_KILLS) level = Math.min(4, Math.max(2, level)) as DuelLevel
  return level
}

export function duelColor(level: DuelLevel): string {
  return `var(--color-duel-${level})`
}
