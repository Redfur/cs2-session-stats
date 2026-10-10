import { Bomb, Clock, Crosshair, History, Scissors } from 'lucide-react'
import type { CSSProperties } from 'react'
import type { Round } from '../../api'
import { otherSide, roundLabel, sideOf } from '../../rounds'
import { cx } from './cx'
import { SideChip } from './SideChip'

const REASON_ICON = { elimination: Crosshair, bomb: Bomb, defuse: Scissors, time: Clock, other: Crosshair, '': Crosshair }

// Группы полосы: половины основного времени по 12 раундов и половины овертаймов по 3.
function groupOf(n: number) {
  if (n <= 12) return 0
  if (n <= 24) return 1
  return 2 + Math.floor((n - 25) / 3)
}

function groupTitle(g: number, first: number, last: number) {
  const range = `${first}–${last}`
  if (g === 0) return `Раунды ${range}`
  if (g === 1) return `Раунды ${range} · смена сторон`
  const ot = g - 2
  return ot % 2 === 0 ? `Овертайм ${ot / 2 + 1} · ${range}` : range
}

interface RoundStripProps {
  rounds: Round[]
  // выбранный раунд виден, только когда события раскрыты
  selected: number | null
  onPick: (n: number) => void
}

// RoundStrip — полоса раундов матча: плашка победителя в ряду его команды (A сверху, B снизу),
// цвет плашки — сторона победителя, иконка — причина конца раунда.
export function RoundStrip({ rounds, selected, onPick }: RoundStripProps) {
  const groups: Round[][] = []
  for (const r of rounds) {
    const g = groupOf(r.number)
    const last = groups[groups.length - 1]
    if (last && groupOf(last[0].number) === g) last.push(r)
    else groups.push([r])
  }
  return (
    <div
      role="group"
      aria-label="Раунды матча: сверху выигранные командой A, снизу — командой B"
      className="flex flex-wrap gap-x-0 gap-y-3.5 px-4 py-3.5 [--rc:22px] [--rs:28px] md:gap-y-4 md:px-5 md:pt-[18px] md:pb-4 md:[--rc:36px] md:[--rs:32px]"
    >
      {groups.map((g) => {
        const sideA = g[0].sideA
        return (
          <div
            key={g[0].number}
            className="flex flex-col gap-2 md:border-l md:border-border md:px-5 md:first:border-l-0 md:first:pl-0"
          >
            <div className="flex items-baseline gap-2 text-over font-bold whitespace-nowrap text-fg-muted uppercase">
              {groupTitle(groupOf(g[0].number), g[0].number, g[g.length - 1].number)}
              <span className="inline-flex items-center gap-1 font-semibold tracking-normal normal-case">
                A <SideChip side={sideA} className="h-4 min-w-[22px] text-[10px]" />
              </span>
              <span className="inline-flex items-center gap-1 font-semibold tracking-normal normal-case">
                B <SideChip side={otherSide(sideA)} className="h-4 min-w-[22px] text-[10px]" />
              </span>
            </div>
            <div className="flex gap-[3px] md:gap-1">
              <div className="grid w-3 items-center justify-items-center gap-y-[3px]" style={ROWS} aria-hidden>
                <span className="size-2 rounded-[2px] bg-team-a" />
                <span />
                <span className="size-2 rounded-[2px] bg-team-b" />
              </div>
              {g.map((r) => (
                <RoundButton key={r.number} round={r} on={selected === r.number} onPick={onPick} />
              ))}
            </div>
          </div>
        )
      })}
    </div>
  )
}

const ROWS: CSSProperties = { gridTemplateRows: 'var(--rs) 18px var(--rs)' }

function RoundButton({ round: r, on, onPick }: { round: Round; on: boolean; onPick: (n: number) => void }) {
  const side = sideOf(r, r.winner)
  const Icon = r.restored ? History : REASON_ICON[r.reason]
  const clutch = r.clutches.some((c) => c.outcome === 'win')
  const label = roundLabel(r)
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      aria-pressed={on}
      onClick={() => onPick(r.number)}
      className="group/rd focus-ring grid w-[var(--rc)] cursor-pointer gap-y-[3px] rounded-[7px] border-0 bg-transparent p-0 text-inherit"
      style={ROWS}
    >
      <span className="col-start-1 row-start-1 rounded-md bg-surface-2 transition-colors group-hover/rd:bg-surface-hover" />
      <span
        className={cx(
          'col-start-1 row-start-2 grid place-items-center rounded-[4px] text-over font-semibold tabular-nums',
          on ? 'bg-primary font-extrabold text-primary-fg' : 'text-fg-faint group-hover/rd:text-fg',
        )}
      >
        {r.number}
      </span>
      <span className="col-start-1 row-start-3 rounded-md bg-surface-2 transition-colors group-hover/rd:bg-surface-hover" />
      <span
        aria-hidden
        className={cx(
          'relative col-start-1 grid place-items-center rounded-md',
          r.winner === 'A' ? 'row-start-1' : 'row-start-3',
          side === 'CT' ? 'text-side-ct' : 'text-side-t',
          r.restored
            ? 'border-[1.5px] border-dashed border-current'
            : side === 'CT'
              ? 'bg-side-ct/15 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--color-side-ct)_45%,transparent)]'
              : 'bg-side-t/15 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--color-side-t)_45%,transparent)]',
        )}
      >
        <Icon className="size-3.5 md:size-4" />
        {clutch && <span className="absolute top-0.5 right-0.5 size-[5px] rounded-full bg-fg md:top-[3px] md:right-[3px]" />}
      </span>
    </button>
  )
}

// RoundLegend — подписи иконок и обозначений полосы.
export function RoundLegend() {
  return (
    <div className="flex flex-wrap items-center gap-x-3.5 gap-y-1.5 text-small text-fg-muted [&>span]:inline-flex [&>span]:items-center [&>span]:gap-1.5 [&_svg]:size-3.5">
      <span>
        <Crosshair aria-hidden />
        все убиты
      </span>
      <span>
        <Bomb aria-hidden />
        взрыв бомбы
      </span>
      <span>
        <Scissors aria-hidden />
        дефьюз
      </span>
      <span>
        <Clock aria-hidden />
        время вышло
      </span>
      <span>
        <SideChip side="CT" />
        <SideChip side="T" />
        цвет — сторона победителя
      </span>
      <span>
        <span className="size-[5px] rounded-full bg-fg" aria-hidden />
        клатч
      </span>
      <span>
        <span className="size-3.5 rounded-[4px] border-[1.5px] border-dashed border-fg-muted" aria-hidden />
        исход восстановлен из счёта
      </span>
    </div>
  )
}
