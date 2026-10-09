import { Link } from 'react-router'
import type { PlayerRow } from '../api'
import { formatPct } from '../metrics'
import type { SortState } from '../sort'
import { rowLinkClass, type Column } from './ui/StatTable'

// Общее описание колонок игрока: итоги сессии, таблицы матча, общая таблица и итоги профиля.

const nameColumn: Column<PlayerRow> = {
  key: 'name',
  label: 'Игрок',
  value: (p) => p.name,
  render: (p) => (
    <Link to={`/players/${p.steamId}`} className={rowLinkClass}>
      {p.name}
    </Link>
  ),
}

const count = (key: string, label: string, title: string, value: (p: PlayerRow) => number, width: number): Column<PlayerRow> => ({
  key,
  label,
  title,
  numeric: true,
  secondary: true,
  value,
  width,
})

const statColumns: Column<PlayerRow>[] = [
  count('k', 'K', 'Убийства', (p) => p.kills, 52),
  count('d', 'D', 'Смерти', (p) => p.deaths, 52),
  count('a', 'A', 'Помощь', (p) => p.assists, 48),
  { key: 'kd', label: 'K/D', title: 'Убийства / смерти', numeric: true, metric: 'kd', value: (p) => p.kd, width: 64 },
  { key: 'adr', label: 'ADR', title: 'Средний урон за раунд', numeric: true, metric: 'adr', value: (p) => p.adr, width: 72 },
  { ...count('hs', 'HS%', 'Доля убийств в голову', (p) => p.hsPct, 60), render: (p) => formatPct(p.hsPct) },
  {
    key: 'kast',
    label: 'KAST',
    title: '% раундов с убийством, помощью, выживанием или разменом',
    numeric: true,
    metric: 'kast',
    value: (p) => p.kastPct,
    width: 64,
  },
  count('k2', '2K', 'Раунды с 2 убийствами', (p) => p.k2, 44),
  count('k3', '3K', 'Раунды с 3 убийствами', (p) => p.k3, 44),
  count('k4', '4K', 'Раунды с 4 убийствами', (p) => p.k4, 44),
  count('k5', '5K', 'Раунды с 5 убийствами', (p) => p.k5, 44),
  {
    ...count('open', 'Open K/D', 'Первые убийства / смерти раунда', (p) => p.openingKills - p.openingDeaths, 84),
    render: (p) => `${p.openingKills}/${p.openingDeaths}`,
  },
  { key: 'rating', label: 'Rating', title: 'HLTV Rating 1.0', numeric: true, metric: 'rating', strong: true, value: (p) => p.rating, width: 80 },
]

// Сводные таблицы: с колонками матчей и побед.
export const PLAYER_TOTAL_COLUMNS: Column<PlayerRow>[] = [
  nameColumn,
  count('m', 'М', 'Матчи', (p) => p.matches ?? 0, 40),
  count('w', 'П', 'Победы', (p) => p.wins ?? 0, 40),
  ...statColumns,
]
export const PLAYER_TOTAL_MIN_WIDTH = 992

// Таблица команды на странице матча.
export const PLAYER_MATCH_COLUMNS: Column<PlayerRow>[] = [nameColumn, ...statColumns]
export const PLAYER_MATCH_MIN_WIDTH = 912

export const RATING_DESC: SortState = { key: 'rating', dir: 'desc' }
