import { useEffect, useRef, useState, type CSSProperties } from 'react'
import { Link } from 'react-router'
import type { DuelCell, DuelPlayer } from '../../api'
import { DUEL_LEVELS, duelColor, duelLevel, duelPct } from '../../duels'
import { cx } from './cx'

interface DuelMatrixProps {
  players: DuelPlayer[]
  cells: DuelCell[]
  // match — игроки сгруппированы по командам, союзники по полю team;
  // session — союзники те, кто ни разу не играл друг против друга (нет ячейки)
  context: 'match' | 'session'
  labelledBy: string
}

type Hover = { r: number; c: number } | null

// Колонка ников и столбцы: ширины из макета, на узком экране уже.
const GRID = '[--du-name:112px] [--du-col:72px] md:[--du-name:168px] md:[--du-col:84px]'
const NAME =
  'sticky left-0 z-[2] flex min-w-0 items-center gap-2 border-r border-border pr-2 pl-3 max-md:shadow-[6px_0_8px_-6px_rgb(0_0_0/0.6)] md:pr-3 md:pl-4'
// разделитель команд в матрице матча
const GROUP_COL = 'shadow-[inset_1px_0_0_var(--color-border-strong)]'
// подсветка строки и столбца под курсором
const CROSS = 'bg-[image:linear-gradient(rgb(231_233_237/0.045),rgb(231_233_237/0.045))]'

// DuelMatrix — матрица «кто кого убивал»: строка — убийца, столбец — жертва.
// В ячейке убийства строки и её доля в паре, полоска — доля, цвет — ступень шкалы.
export function DuelMatrix({ players, cells, context, labelledBy }: DuelMatrixProps) {
  const [hover, setHover] = useState<Hover>(null)
  const ref = useRef<HTMLDivElement>(null)
  const byPair = new Map(cells.map((c) => [`${c.killerId}|${c.victimId}`, c]))
  const n = players.length
  const grouped = context === 'match'
  const firstB = grouped ? players.findIndex((p) => p.team === 'B') : -1
  const countA = grouped ? players.filter((p) => p.team === 'A').length : 0

  // касание вне матрицы закрывает подсказку
  useEffect(() => {
    if (!hover) return
    const close = (e: PointerEvent) => {
      if (!ref.current?.contains(e.target as Node)) setHover(null)
    }
    document.addEventListener('pointerdown', close)
    return () => document.removeEventListener('pointerdown', close)
  }, [hover])

  const row = 'grid border-b border-border last:border-b-0 grid-cols-[var(--du-name)_repeat(var(--du-n),minmax(var(--du-col),1fr))]'
  const style = { '--du-n': n, minWidth: `calc(var(--du-name) + ${n} * var(--du-col))` } as CSSProperties

  return (
    <div className="relative max-md:after:pointer-events-none max-md:after:absolute max-md:after:inset-y-0 max-md:after:right-0 max-md:after:z-[4] max-md:after:w-5 max-md:after:bg-[linear-gradient(90deg,transparent,var(--color-surface))]">
      <div className="relative overflow-x-auto [-webkit-overflow-scrolling:touch]" onMouseLeave={() => setHover(null)}>
        <div ref={ref} role="table" aria-labelledby={labelledBy} className={cx(GRID, 'text-table tabular-nums')} style={style}>
          {grouped && (
            <div className={row} role="presentation">
              <div className={cx(NAME, 'h-8 bg-surface-2')} />
              <TeamBand team="A" span={countA} />
              <TeamBand team="B" span={n - countA} className={GROUP_COL} />
            </div>
          )}
          <div className={row} role="row">
            <div className={cx(NAME, 'bg-surface-2')} role="columnheader">
              <span className="text-over font-semibold whitespace-nowrap text-fg-faint normal-case tracking-normal max-md:hidden">
                убийца ↓ · жертва →
              </span>
            </div>
            {players.map((p, j) => (
              <div
                key={p.steamId}
                role="columnheader"
                title={p.name}
                className={cx(
                  'flex h-10 min-w-0 items-end justify-end px-2 pb-2.5 text-small font-semibold transition-colors md:px-2.5',
                  hover?.c === j ? 'bg-surface-hover text-fg' : 'bg-surface-2 text-fg-muted',
                  j === firstB && GROUP_COL,
                )}
              >
                <span className="block min-w-0 truncate">{p.name}</span>
              </div>
            ))}
          </div>
          {players.map((killer, i) => (
            <div key={killer.steamId} role="row" className={cx(row, i === firstB && 'border-t border-t-border-strong')}>
              <div role="rowheader" className={cx(NAME, hover?.r === i ? 'bg-surface-hover' : 'bg-surface')}>
                {grouped && (
                  <span className={cx('size-2.5 flex-none rounded-[3px]', killer.team === 'A' ? 'bg-team-a' : 'bg-team-b')} aria-hidden />
                )}
                <Link
                  to={`/players/${killer.steamId}`}
                  title={killer.name}
                  className="focus-ring min-w-0 truncate font-semibold text-fg hover:text-accent-hover"
                >
                  {killer.name}
                </Link>
              </div>
              {players.map((victim, j) => (
                <Cell
                  key={victim.steamId}
                  killer={killer}
                  victim={victim}
                  cell={i === j ? undefined : byPair.get(`${killer.steamId}|${victim.steamId}`)}
                  ally={i === j || (grouped ? killer.team === victim.team : !byPair.has(`${killer.steamId}|${victim.steamId}`))}
                  context={context}
                  crossed={hover !== null && (hover.r === i || hover.c === j)}
                  active={hover?.r === i && hover?.c === j}
                  tipBelow={i < 2}
                  tipRight={j < n / 2}
                  groupStart={j === firstB}
                  onHover={() => setHover({ r: i, c: j })}
                />
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function TeamBand({ team, span, className }: { team: 'A' | 'B'; span: number; className?: string }) {
  return (
    <div
      className={cx(
        'flex h-8 items-center gap-2 px-2.5 text-small font-bold whitespace-nowrap',
        team === 'A' ? 'bg-team-a/14' : 'bg-team-b/13',
        className,
      )}
      style={{ gridColumn: `span ${span}` }}
    >
      <span className={cx('size-2.5 flex-none rounded-[3px]', team === 'A' ? 'bg-team-a' : 'bg-team-b')} aria-hidden />
      Команда {team}
    </div>
  )
}

interface CellProps {
  killer: DuelPlayer
  victim: DuelPlayer
  cell?: DuelCell
  ally: boolean
  context: 'match' | 'session'
  crossed: boolean
  active: boolean
  tipBelow: boolean
  tipRight: boolean
  groupStart: boolean
  onHover: () => void
}

function Cell({ killer, victim, cell, ally, context, crossed, active, tipBelow, tipRight, groupStart, onHover }: CellProps) {
  const base = cx(
    'relative flex h-11 min-w-0 flex-col items-end justify-center gap-[5px] px-2 whitespace-nowrap md:h-12 md:gap-1.5 md:px-2.5',
    groupStart && 'shadow-[inset_1px_0_0_var(--color-border-strong)]',
    crossed && CROSS,
  )
  if (ally || !cell) {
    const self = killer.steamId === victim.steamId
    const sr = self ? '—' : `${killer.name} и ${victim.name}: ${context === 'match' ? 'в одной команде' : 'весь вечер в одной команде'}`
    return (
      <div role="cell" className={cx(base, 'bg-bg')}>
        <span className="sr-only">{sr}</span>
        <span className="font-medium text-fg-faint" aria-hidden>
          —
        </span>
      </div>
    )
  }

  const { kills, deaths } = cell
  const level = duelLevel(kills, deaths)
  const line1 = `${killer.name} против ${victim.name}: ${kills} : ${deaths}`
  const line2 = context === 'match' ? 'в этом матче' : `карт против: ${cell.maps}`
  const lv = level ? duelColor(level) : undefined
  return (
    <div
      role="cell"
      className={cx(base, 'cursor-default bg-surface', active && 'z-[3] shadow-[inset_0_0_0_1px_var(--color-border-hover)]')}
      onMouseEnter={onHover}
      onClick={onHover}
    >
      <span className="sr-only">
        {line1}, {line2}
      </span>
      <span className="flex items-baseline gap-1" aria-hidden>
        {level ? (
          <>
            <span className="font-bold text-fg">{kills}</span>
            <span className="text-fg-faint">·</span>
            <span className="font-medium text-fg-muted">{duelPct(kills, deaths)}%</span>
          </>
        ) : (
          <span className="font-medium text-fg-muted">0</span>
        )}
      </span>
      <ShareBar color={lv} share={level ? kills / (kills + deaths) : 0} />
      {active && (
        <span
          aria-hidden
          className={cx(
            'pointer-events-none absolute z-[6] flex flex-col gap-0.5 rounded-md border border-border-strong bg-surface-2 px-3 py-2 text-left whitespace-nowrap shadow-[var(--shadow-overlay)]',
            tipBelow ? 'top-[calc(100%+6px)]' : 'bottom-[calc(100%+6px)]',
            tipRight ? 'left-0' : 'right-0',
          )}
        >
          <b className="font-bold text-fg">{line1}</b>
          <span className="text-small text-fg-muted">{line2}</span>
        </span>
      )}
    </div>
  )
}

// ShareBar — полоска доли: подложка тонирована ступенью, сплошная часть — доля, риска — 50%.
function ShareBar({ color, share }: { color?: string; share: number }) {
  return (
    <span
      aria-hidden
      className="relative block h-1 w-full rounded-[2px] after:absolute after:-top-0.5 after:left-1/2 after:h-2 after:w-px after:bg-border-hover"
      style={{
        background: color ? `color-mix(in srgb, ${color} 28%, var(--color-surface))` : 'var(--color-surface-hover)',
      }}
    >
      {color && <span className="absolute inset-y-0 left-0 rounded-[2px]" style={{ width: `${(share * 100).toFixed(1)}%`, background: color }} />}
    </span>
  )
}

// DuelLegend — легенда шкалы и обозначение «—».
export function DuelLegend({ allies }: { allies: string }) {
  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-small text-fg-muted">
      <span
        role="img"
        aria-label={`Доля убийств в паре: ${DUEL_LEVELS.map((l) => l.name).join(', ')}`}
        className="inline-flex flex-wrap items-center gap-x-3 gap-y-1.5 tabular-nums"
      >
        {DUEL_LEVELS.map((l) => (
          <span key={l.level} className="inline-flex items-center gap-1.5" aria-hidden>
            <i className="block h-1 w-5 rounded-[2px]" style={{ background: duelColor(l.level) }} />
            {l.label}
          </span>
        ))}
      </span>
      <span className="inline-flex items-center gap-2">
        <span className="font-bold text-fg-faint">—</span>
        {allies}
      </span>
    </div>
  )
}
