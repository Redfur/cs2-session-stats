import type { PlayerRow } from '../api'
import { ovrChangeText } from '../metrics'
import { playerNameColumn } from './playerColumns'
import { OvrChange, ScoreBadge, VsUsual } from './ui/Rating'
import type { Column } from './ui/StatTable'

// Колонки вида итогов сессии «Оценки»: оценка за вечер, сравнение с обычным уровнем и OVR до и после.

const FIRST_SESSION = 'Первая сессия игрока — сравнивать не с чем'

const score: Column<PlayerRow> = {
  key: 'score',
  label: 'Оценка',
  title: 'Оценка за сессию, 0–100: Rating 2.0 и вклад в исход раундов',
  numeric: true,
  value: (p) => p.score ?? -1,
  render: (p) => p.score !== undefined && <ScoreBadge score={p.score} />,
}

const vsUsual = (number: boolean): Column<PlayerRow> => ({
  key: 'vs',
  label: number ? 'К обычному' : '',
  name: 'к обычному',
  title: 'Оценка за сессию минус OVR перед ней',
  numeric: true,
  value: (p) => p.change?.vsUsual ?? -1000,
  render: (p) => <VsUsual delta={p.change?.vsUsual} usual={p.change?.before} missing={FIRST_SESSION} number={number} />,
})

const ovr = (compact: boolean): Column<PlayerRow> => ({
  key: 'ovr',
  label: 'OVR за вечер',
  title: 'OVR до и после вечера',
  numeric: true,
  align: 'left',
  value: (p) => p.change?.after ?? -1,
  render: (p) => {
    const c = p.change
    if (!c) return null
    return (
      <OvrChange
        before={c.before}
        after={c.after}
        delta={c.delta}
        calibrating={c.calibrating}
        matches={c.matches}
        calibration={c.calibrationMatches}
        compact={compact}
        title={c.before !== undefined && c.delta !== undefined ? ovrChangeText(c.before, c.after, c.delta) : undefined}
      />
    )
  },
})

export const SCORE_COLUMNS: Column<PlayerRow>[] = [
  playerNameColumn,
  { key: 'm', label: 'М', title: 'Матчи', numeric: true, secondary: true, width: 40, value: (p) => p.matches ?? 0 },
  { key: 'rating', label: 'Rating', title: 'HLTV Rating 2.0', numeric: true, metric: 'rating', strong: true, width: 76, value: (p) => p.rating },
  { ...score, width: 76 },
  { ...vsUsual(true), width: 112 },
  { ...ovr(false), width: 230 },
]
export const SCORE_MIN_WIDTH = 660

// Узкий экран: игрок, оценка, иконка сравнения и OVR «стало + дельта»; М и Rating скрыты.
export const SCORE_COLUMNS_NARROW: Column<PlayerRow>[] = [
  { ...playerNameColumn, width: 100 },
  { ...score, width: 64 },
  { ...vsUsual(false), width: 44 },
  ovr(true),
]
