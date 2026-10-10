import { ArrowRight, Info } from 'lucide-react'
import { useId, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { Match, MatchExt, PlayerRow, Side } from '../api'
import { plural } from '../format'
import { otherSide, qualityNotes, sumWeapons } from '../rounds'
import { sortRows, useSort, type SortState } from '../sort'
import { weaponColumns } from './weaponColumns'
import { Alert } from './ui/Alert'
import { Button } from './ui/Button'
import { Card, CardFooter, CardHeader } from './ui/Card'
import { inputClass, Label } from './ui/Field'
import { RoundDetails, type RoundPlayer } from './ui/RoundDetails'
import { RoundLegend, RoundStrip } from './ui/RoundStrip'
import { SideChip } from './ui/SideChip'
import { StatTable } from './ui/StatTable'
import { cx } from './ui/cx'

// MatchRounds — полоса раундов под счётом, легенда и раскрываемые события раунда.
export function MatchRounds({ ext, players }: { ext: MatchExt; players: PlayerRow[] }) {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState<number | null>(null)
  const bodyId = useId()
  const rounds = ext.rounds
  if (rounds.length === 0) return null
  const current = rounds.find((r) => r.number === selected) ?? rounds[rounds.length - 1]
  const byId = new Map<string, RoundPlayer>(players.map((p) => [p.steamId, { name: p.name, team: p.team ?? 'A' }]))
  return (
    <>
      <div className="mt-6 border-t border-border md:mt-7 md:flex md:justify-center">
        <RoundStrip
          rounds={rounds}
          selected={open ? current.number : null}
          onPick={(n) => {
            setSelected(n)
            setOpen(true)
          }}
        />
      </div>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 border-t border-border px-4 py-3.5 md:px-5">
        <span className="text-small text-fg-muted tabular-nums">
          {plural(rounds.length, ['раунд', 'раунда', 'раундов'])} · Команда A начала матч за{' '}
          <SideChip side={rounds[0].sideA} />
        </span>
        <div className="min-w-0 flex-[1_1_420px]">
          <RoundLegend />
        </div>
        <Button variant="ghost" size="sm" aria-expanded={open} aria-controls={bodyId} onClick={() => setOpen((o) => !o)}>
          {open ? 'Скрыть события' : 'События раундов'}
        </Button>
      </div>
      {open && <RoundDetails id={bodyId} round={current} players={byId} />}
    </>
  )
}

// TeamSides — стороны команды по половинам: CT → T.
export function TeamSides({ first }: { first: Side }) {
  return (
    <span
      className="inline-flex items-center gap-1 text-fg-faint"
      title={`Раунды 1–12 за ${first}, с 13-го за ${otherSide(first)}`}
    >
      <SideChip side={first} />
      <ArrowRight className="size-3" aria-hidden />
      <SideChip side={otherSide(first)} />
    </span>
  )
}

// MatchQualityAlert — матч готов, но данных меньше: информационное сообщение, а не ошибка.
export function MatchQualityAlert({ ext, match }: { ext: MatchExt; match: Match }) {
  const notes = qualityNotes(ext, match)
  if (notes.length === 0) return null
  return (
    <Alert tone="info" icon={<Info aria-hidden />} title="Запись неполная.">
      {notes.join(' ')}
    </Alert>
  )
}

// ── Оружие матча ──

const WEAPONS_DEFAULT: SortState = { key: 'kills', dir: 'desc' }

// MatchWeaponsCard — оружие всех игроков матча или одного; выбор хранится в адресе (wp).
export function MatchWeaponsCard({ ext, players }: { ext: MatchExt; players: PlayerRow[] }) {
  const [params, setParams] = useSearchParams()
  const selectId = useId()
  const wp = params.get('wp') ?? ''
  const player = players.find((p) => p.steamId === wp)
  const columns = ext.quality.noDamageEvents ? NO_DAMAGE_COLUMNS : FULL_COLUMNS
  const sw = useSort('sw', columns, WEAPONS_DEFAULT)
  const rows = sumWeapons(player ? ext.weapons.filter((w) => w.steamId === player.steamId) : ext.weapons)
  const order = [...players].sort((a, b) => (a.team ?? '').localeCompare(b.team ?? '') || b.rating - a.rating)
  return (
    <Card aria-labelledby="mw-h">
      <CardHeader
        title="Оружие"
        titleId="mw-h"
        note={player ? `только ${player.name}` : 'все игроки матча'}
        aside={
          <div className="flex items-center gap-2.5">
            <Label htmlFor={selectId}>Игрок</Label>
            <select
              id={selectId}
              className={cx(inputClass, 'w-auto min-w-40 cursor-pointer pr-8')}
              value={player ? player.steamId : ''}
              onChange={(e) =>
                setParams(
                  (prev) => {
                    const p = new URLSearchParams(prev)
                    if (e.target.value) p.set('wp', e.target.value)
                    else p.delete('wp')
                    return p
                  },
                  { replace: true },
                )
              }
            >
              <option value="">Все игроки</option>
              {order.map((p) => (
                <option key={p.steamId} value={p.steamId}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>
        }
      />
      {rows.length > 0 ? (
        <StatTable
          columns={columns}
          rows={sortRows(rows, columns, sw.sort)}
          rowKey={(r) => r.weapon}
          sort={sw.sort}
          onSort={sw.toggle}
          labelledBy="mw-h"
          minWidth={780}
        />
      ) : (
        <p className="m-0 px-4 py-5 text-[13px] text-fg-muted md:px-5">Убийств и урона из оружия нет.</p>
      )}
      <CardFooter className="text-small text-fg-muted">
        Выстрелы и попадания — счётчики, процент точности не показываем. Убийства гранатами в эту таблицу не входят.
      </CardFooter>
    </Card>
  )
}

const FULL_COLUMNS = weaponColumns(true)
const NO_DAMAGE_COLUMNS = weaponColumns(false)
