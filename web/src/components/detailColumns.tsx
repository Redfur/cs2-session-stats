import { Link } from 'react-router'
import type { DetailRow, PlayerRow } from '../api'
import { fmt1, fmtPct, fmtSigned } from '../ext'
import { NotComputed } from './ui/ExtValue'
import { MetricValue } from './ui/MetricValue'
import { rowLinkClass, type Column } from './ui/StatTable'

// Подробные колонки игрока: базовая строка и расширенные счётчики. Новые показатели шкалой не окрашиваются.
export type DetailPlayer = PlayerRow & { d?: DetailRow }

// Причины прочерка: у игрока нет расширенных данных, в демке нет событий урона или ослепления.
export interface DetailReasons {
  old: string
  noDamage: string
  noFlash: string
}

const num = (v: number | null | undefined) => v ?? -Infinity

function value(p: DetailPlayer, v: number | null | undefined, reason: string, format: (v: number) => string) {
  if (!p.d) return null
  return v == null ? <NotComputed reason={reason} /> : format(v)
}

export function detailColumns(kind: 'match' | 'session', r: DetailReasons): Column<DetailPlayer>[] {
  const old = (p: DetailPlayer, content: React.ReactNode) => (p.d ? content : <NotComputed reason={r.old} />)
  const cols: (Column<DetailPlayer> | false)[] = [
    {
      key: 'name',
      label: 'Игрок',
      value: (p) => p.name,
      render: (p) => (
        <Link to={`/players/${p.steamId}`} className={rowLinkClass}>
          {p.name}
        </Link>
      ),
    },
    kind === 'session' && {
      key: 'm',
      label: 'М',
      title: 'Матчи с подробными данными',
      numeric: true,
      secondary: true,
      width: 40,
      value: (p) => p.d?.matches ?? 0,
    },
    { key: 'adr', label: 'ADR', title: 'Нанесённый урон за раунд', numeric: true, metric: 'adr', width: 72, value: (p) => p.adr },
    {
      key: 'taken',
      label: 'Получ./р',
      title: 'Полученный от противников урон за раунд',
      numeric: true,
      secondary: true,
      width: 84,
      value: (p) => num(p.d?.takenPerRound),
      render: (p) => old(p, value(p, p.d?.takenPerRound, r.noDamage, fmt1)),
    },
    {
      key: 'diff',
      label: 'Разница',
      title: 'Нанесённый − полученный урон за раунд',
      numeric: true,
      width: 80,
      value: (p) => num(p.d?.diffPerRound),
      render: (p) => old(p, value(p, p.d?.diffPerRound, r.noDamage, fmtSigned)),
    },
    {
      key: 'tk',
      label: 'Разменял',
      title: 'Убийства в размен: отомстил за союзника не позже 5 с',
      numeric: true,
      width: 88,
      value: (p) => num(p.d?.tradeKills),
      render: (p) => old(p, p.d?.tradeKills),
    },
    {
      key: 'td',
      label: 'Разменяли',
      title: 'Смерти, за которые союзник отомстил не позже 5 с',
      numeric: true,
      secondary: true,
      width: 96,
      value: (p) => num(p.d?.tradedDeaths),
      render: (p) => old(p, p.d?.tradedDeaths),
    },
    {
      key: 'cl',
      label: 'Клатчи',
      title: 'Выигранные клатчи / все попытки',
      numeric: true,
      width: 76,
      value: (p) => num(p.d?.clutchWins),
      render: (p) =>
        old(
          p,
          p.d && (
            <span title={p.d.clutchAttempts ? `Выиграно ${p.d.clutchWins} из ${p.d.clutchAttempts}` : 'Клатчей не было'}>
              {p.d.clutchWins} / {p.d.clutchAttempts}
            </span>
          ),
        ),
    },
    kind === 'session' && {
      key: 'nades',
      label: 'Гранаты/карта',
      title: 'Урон HE и огнём в среднем за карту',
      numeric: true,
      width: 116,
      value: (p) => num(p.d?.grenadeDamage),
      render: (p) => old(p, value(p, p.d?.grenadeDamage, r.noDamage, fmt1)),
    },
    {
      key: 'fa',
      label: 'Флеш-асс.',
      title: 'Противника добили, пока он был ослеплён флешкой игрока',
      numeric: true,
      secondary: true,
      width: 88,
      value: (p) => num(p.d?.flashAssists),
      render: (p) => old(p, p.d?.flashAssists),
    },
    kind === 'match' && {
      key: 'flashed',
      label: 'Ослепил',
      title: 'Пары «флешка — противник», не уникальные люди',
      numeric: true,
      secondary: true,
      width: 80,
      value: (p) => num(p.d?.flashed),
      render: (p) => old(p, value(p, p.d?.flashed, r.noFlash, (v) => String(Math.round(v)))),
    },
    kind === 'match' && {
      key: 'he',
      label: 'Урон HE',
      numeric: true,
      secondary: true,
      width: 84,
      value: (p) => num(p.d?.heDamage),
      render: (p) => old(p, value(p, p.d?.heDamage, r.noDamage, (v) => String(Math.round(v)))),
    },
    kind === 'match' && {
      key: 'fire',
      label: 'Урон огнём',
      title: 'Molotov и Incendiary вместе',
      numeric: true,
      secondary: true,
      width: 100,
      value: (p) => num(p.d?.fireDamage),
      render: (p) => old(p, value(p, p.d?.fireDamage, r.noDamage, (v) => String(Math.round(v)))),
    },
    kind === 'match' && {
      key: 'smokes',
      label: 'Смоки',
      numeric: true,
      secondary: true,
      width: 68,
      value: (p) => num(p.d?.smokes),
      render: (p) => old(p, value(p, p.d?.smokes, r.old, (v) => String(Math.round(v)))),
    },
    {
      key: 'surv',
      label: 'Выжил',
      title: 'Доля раундов, в которых дожил до конца',
      numeric: true,
      width: 72,
      value: (p) => num(p.d?.survivedPct),
      render: (p) => old(p, value(p, p.d?.survivedPct, 'Раундов не было', fmtPct)),
    },
    kind === 'session' && {
      key: 'rating',
      label: 'Rating',
      title: 'HLTV Rating 1.0',
      numeric: true,
      metric: 'rating',
      strong: true,
      width: 80,
      value: (p) => p.rating,
      render: (p) => <MetricValue metric="rating" value={p.rating} strong />,
    },
  ]
  return cols.filter((c): c is Column<DetailPlayer> => c !== false)
}

// joinDetails добавляет к строкам игроков их подробные счётчики.
export function joinDetails(players: PlayerRow[], details: DetailRow[] | undefined): DetailPlayer[] {
  const by = new Map((details ?? []).filter((d) => d.matches > 0).map((d) => [d.steamId, d]))
  return players.map((p) => ({ ...p, d: by.get(p.steamId) }))
}
