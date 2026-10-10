// Раунды и расширенные данные матча: подписи, стороны, длительность, качество записи.

import type { Match, MatchExt, Round, RoundReason, Side, WeaponRow } from './api'
import { plural } from './format'

export const REASON_TEXT: Record<RoundReason, string> = {
  elimination: 'все убиты',
  bomb: 'взрыв бомбы',
  defuse: 'дефьюз',
  time: 'время вышло',
  other: 'другая причина',
  '': '',
}

export const otherSide = (s: Side): Side => (s === 'CT' ? 'T' : 'CT')
export const sideOf = (r: Round, team: 'A' | 'B'): Side => (team === 'A' ? r.sideA : otherSide(r.sideA))

export function roundLabel(r: Round) {
  const side = sideOf(r, r.winner)
  const won = r.clutches.find((c) => c.outcome === 'win')
  return (
    `Раунд ${r.number}: Команда ${r.winner} (${side})` +
    (r.restored ? ', исход восстановлен из счёта' : `, ${REASON_TEXT[r.reason]}`) +
    (won ? `, клатч 1v${won.vs}` : '')
  )
}

export function mmss(sec: number) {
  const s = Math.max(0, Math.round(sec))
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

// matchDuration — длительность матча под счётом; у оборванной демки — «не меньше».
export function matchDuration(ext: MatchExt | null) {
  if (!ext || ext.status !== 'complete' || ext.durationSec == null) return undefined
  const text = mmss(ext.durationSec)
  return ext.quality.restoredRound
    ? { text: `≥ ${text}`, title: 'Демка оборвалась: длительность известна только до обрыва записи' }
    : { text, title: 'Длительность матча' }
}

// qualityNotes — что именно не так с записью демки.
export function qualityNotes(ext: MatchExt, match: Match): string[] {
  const notes: string[] = []
  const q = ext.quality
  if (q.firstRound > 1)
    notes.push(
      `Запись начинается с ${q.firstRound}-го раунда. Раунды 1–${q.firstRound - 1} не записаны, статистика игроков — по ${plural(match.rounds, ['записанному раунду', 'записанным раундам', 'записанным раундам'])}.`,
    )
  if (q.restoredRound) {
    const last = ext.rounds[ext.rounds.length - 1]
    notes.push(
      `Демка оборвалась в ${last?.number ?? match.rounds}-м раунде. Его победитель восстановлен из итогового счёта ${match.scoreA} : ${match.scoreB}. Ход раунда, урон и гранаты за него не посчитаны — подробные колонки считаются по ${plural(ext.extRounds, ['раунду', 'раундам', 'раундам'])}.`,
    )
  }
  if (q.noDamageEvents)
    notes.push('В демке нет событий урона. Полученный урон, разница урона и урон гранат для этого матча не посчитаны.')
  if (q.noFlashEvents) notes.push('В демке нет событий ослепления. Ослепления для этого матча не посчитаны.')
  return notes
}

export function isIncomplete(ext: MatchExt | null, match: Match) {
  return !!ext && ext.status === 'complete' && qualityNotes(ext, match).length > 0
}

export function sumWeapons(rows: WeaponRow[]): WeaponRow[] {
  const by = new Map<string, WeaponRow>()
  for (const r of rows) {
    const a = by.get(r.weapon) ?? { weapon: r.weapon, kills: 0, hsKills: 0, damage: 0, shots: 0, hits: 0, hsHits: 0, hsKillsPct: null, hsHitsPct: null }
    a.kills += r.kills
    a.hsKills += r.hsKills
    a.damage += r.damage
    a.shots += r.shots
    a.hits += r.hits
    a.hsHits += r.hsHits
    by.set(r.weapon, a)
  }
  return [...by.values()].map((a) => ({
    ...a,
    hsKillsPct: a.kills ? Math.round((100 * a.hsKills) / a.kills) : null,
    hsHitsPct: a.hits ? Math.round((100 * a.hsHits) / a.hits) : null,
  }))
}
