import { ArrowRight, Minus, TrendingDown, TrendingUp } from 'lucide-react'
import type { CSSProperties, ReactNode } from 'react'
import type { TeamOVR } from '../../api'
import { CoverageChip } from './ExtValue'
import { formatDelta, scoreTier, vsUsualText } from '../../metrics'
import { cx } from './cx'

// Компоненты оценки и OVR (доска «16 Оценка и OVR»).

const SIZES = {
  sm: 'h-[22px] min-w-[34px] rounded-[6px] px-1.5 text-[13px] leading-4',
  lg: 'h-8 min-w-[46px] rounded-md px-[9px] text-[18px] leading-[22px]',
  xl: 'h-[42px] min-w-14 rounded-[10px] px-3 text-[24px] leading-7 tracking-[-0.02em] md:h-[50px] md:min-w-[68px] md:text-[30px] md:leading-[34px]',
} as const

// ScoreBadge — плашка оценки или OVR. Ступень подкрашивает рамку и фон, число видно всегда.
// В калибровке — пунктирная рамка без заливки и приглушённый текст.
export function ScoreBadge({
  score,
  size = 'sm',
  calibrating = false,
  title,
}: {
  score: number
  size?: keyof typeof SIZES
  calibrating?: boolean
  title?: string
}) {
  const tier = scoreTier(score)
  return (
    <span
      title={title}
      data-tier={tier}
      style={{ '--c': `var(--color-ovr-${tier})` } as CSSProperties}
      className={cx(
        'inline-flex flex-none items-center justify-center border font-extrabold whitespace-nowrap tabular-nums',
        SIZES[size],
        calibrating
          ? 'border-dashed border-border-hover bg-transparent text-fg-muted'
          : tier === 5
            ? 'border-(--c) bg-(--c) text-bg'
            : 'border-[color-mix(in_srgb,var(--c)_42%,transparent)] bg-[color-mix(in_srgb,var(--c)_8%,transparent)] text-(--c)',
      )}
    >
      {score}
    </span>
  )
}

// CalibrationCount — счётчик калибровки «5/10» рядом с OVR.
export function CalibrationCount({ matches, of }: { matches: number; of: number }) {
  return (
    <span className="cursor-help text-over font-semibold text-fg-muted" title={`Калибровка: сыграно ${matches} из ${of} матчей`}>
      {matches}/{of}
    </span>
  )
}

// CalibrationTag — метка «калибровка» вместо изменения OVR в первой сессии игрока.
export function CalibrationTag({ title = 'Первая сессия игрока — OVR ещё не с чем сравнить' }: { title?: string }) {
  return (
    <span
      title={title}
      className="inline-flex h-5 cursor-help items-center rounded-full px-2 text-over font-semibold whitespace-nowrap text-fg-muted shadow-[inset_0_0_0_1px_var(--color-border-hover)]"
    >
      калибровка
    </span>
  )
}

const TRENDS = {
  up: { Icon: TrendingUp, cls: 'text-metric-good bg-metric-good/13' },
  down: { Icon: TrendingDown, cls: 'text-metric-bad bg-metric-bad/14' },
  same: { Icon: Minus, cls: 'text-status-neutral bg-status-neutral/13' },
} as const

// VsUsual — лучше или хуже обычного: иконка тренда, число и обычный уровень — в подсказке.
// С number — иконка и разница числом (колонка «К обычному»). Без сравнения — прочерк с причиной.
export function VsUsual({
  delta,
  usual,
  missing,
  number = false,
  numberClassName,
  className,
}: {
  delta?: number
  usual?: number
  // причина, почему сравнения нет
  missing: string
  number?: boolean
  // классы числа: например, скрыть его на узком экране
  numberClassName?: string
  className?: string
}) {
  if (delta === undefined || usual === undefined) {
    return (
      <span title={missing} aria-label={missing} className="inline-grid size-[22px] flex-none cursor-help place-items-center text-fg-faint">
        —
      </span>
    )
  }
  const t = TRENDS[delta > 0 ? 'up' : delta < 0 ? 'down' : 'same']
  const text = vsUsualText(delta, usual)
  return (
    <span
      title={text}
      aria-label={text}
      role="img"
      className={cx(
        'inline-flex h-[22px] flex-none cursor-help items-center justify-center gap-[3px] rounded-[5px] text-small font-bold whitespace-nowrap tabular-nums',
        number ? 'min-w-[22px] pr-1.5 pl-1' : 'w-[22px]',
        t.cls,
        className,
      )}
    >
      <t.Icon className="size-3.5" strokeWidth={2.25} aria-hidden />
      {number && <span className={numberClassName}>{formatDelta(delta)}</span>}
    </span>
  )
}

// ScoreWithUsual — оценка с иконкой сравнения слева, как в колонках «Оценка».
export function ScoreWithUsual({ score, delta, usual, missing }: { score?: number; delta?: number; usual?: number; missing: string }) {
  if (score === undefined) return <span className="text-fg-faint">—</span>
  return (
    <span className="inline-flex items-center justify-end gap-1.5">
      <VsUsual delta={delta} usual={usual} missing={missing} />
      <ScoreBadge score={score} />
    </span>
  )
}

// Delta — изменение OVR в нейтральной рамке: цвет занят сравнением с обычным уровнем.
export function Delta({ value, className }: { value: number; className?: string }) {
  return (
    <span
      className={cx(
        'inline-flex h-5 flex-none items-center rounded-sm px-1.5 text-small font-bold tabular-nums shadow-[inset_0_0_0_1px_var(--color-border-strong)]',
        value === 0 ? 'text-fg-muted' : 'text-fg',
        className,
      )}
    >
      {formatDelta(value)}
    </span>
  )
}

interface OvrChangeProps {
  before?: number
  after: number
  delta?: number
  calibrating: boolean
  // матчей после сессии и порог калибровки — для счётчика «8/10»
  matches: number
  calibration: number
  // компактно: «стало + дельта» (узкий экран)
  compact?: boolean
  title?: string
}

// OvrChange — OVR до и после сессии: «74 → 76 +2». В первой сессии — OVR и метка «калибровка».
export function OvrChange({ before, after, delta, calibrating, matches, calibration, compact = false, title }: OvrChangeProps) {
  if (before === undefined || delta === undefined) {
    return (
      <span className="inline-flex items-center gap-1.5">
        <ScoreBadge score={after} calibrating={calibrating} />
        <CalibrationTag />
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap tabular-nums" title={title}>
      {!compact && (
        <>
          <span className="font-semibold text-fg-muted">{before}</span>
          <ArrowRight className="size-3.5 text-fg-faint" aria-label="→" />
        </>
      )}
      <ScoreBadge score={after} calibrating={calibrating} />
      <Delta value={delta} />
      {calibrating && !compact && <CalibrationCount matches={matches} of={calibration} />}
    </span>
  )
}

// OvrTooltip — раскрывающаяся подсказка у изменения OVR: оценка за сессию, обычный уровень
// и место под разложение изменения по слагаемым (появится отдельным change).
export function OvrTooltip({
  trigger,
  before,
  after,
  delta,
  score,
  align = 'left',
}: {
  trigger: ReactNode
  before: number
  after: number
  delta: number
  score: number
  align?: 'left' | 'right'
}) {
  return (
    <button type="button" className="group/tip focus-ring relative inline-flex cursor-help rounded-[6px] border-0 bg-transparent p-0 text-left font-[inherit] text-inherit">
      {trigger}
      <span
        role="tooltip"
        className={cx(
          'absolute top-[calc(100%+8px)] z-[6] hidden w-[260px] cursor-default flex-col gap-2.5 rounded-md border border-border-strong bg-surface-2 px-3.5 pt-3 pb-3.5 text-small font-medium whitespace-normal text-fg-muted shadow-overlay md:w-[300px]',
          'group-hover/tip:flex group-focus-visible/tip:flex',
          align === 'left' ? 'left-0' : 'right-0',
        )}
      >
        <span className="flex items-center justify-between gap-2.5 text-[13px] leading-[18px] font-bold text-fg">
          <span>
            OVR за сессию: {before} → {after}
          </span>
          <Delta value={delta} />
        </span>
        <span className="flex items-center justify-between gap-3 text-fg">
          <span>Оценка за сессию</span>
          <span className="inline-flex items-center gap-1.5">
            <VsUsual delta={score - before} usual={before} missing="" />
            <ScoreBadge score={score} />
          </span>
        </span>
        <span className="flex items-center justify-between gap-3 text-fg">
          <span>Обычно — OVR перед сессией</span>
          <b className="font-bold tabular-nums">{before}</b>
        </span>
        <span className="flex flex-col gap-1.5 border-t border-border pt-2.5">
          <span className="text-over font-bold uppercase text-fg-muted">Из чего сложилось</span>
          <span className="flex flex-col gap-[7px] rounded-[6px] border border-dashed border-border-strong p-2.5" aria-hidden>
            <i className="block h-2 w-[78%] rounded-[3px] bg-surface-hover" />
            <i className="block h-2 w-[58%] rounded-[3px] bg-surface-hover" />
            <i className="block h-2 w-[66%] rounded-[3px] bg-surface-hover" />
          </span>
          <span className="text-small text-fg-faint">Разложение появится позже: убийства, урон, KAST…</span>
        </span>
      </span>
    </button>
  )
}

// RatingTile — плитка профиля с оценкой: OVR, «Среднее за 20», последняя сессия.
export function RatingTile({ label, children, sub, className }: { label: string; children: ReactNode; sub?: ReactNode; className?: string }) {
  return (
    <div className={cx('flex min-w-0 flex-col gap-1.5 rounded-lg border border-border bg-surface px-[18px] py-4', className)}>
      <span className="text-over font-bold uppercase text-fg-muted">{label}</span>
      <div className="flex min-h-[42px] flex-wrap items-center gap-x-3 gap-y-2 md:min-h-[50px]">{children}</div>
      {sub && <span className="text-small text-fg-muted">{sub}</span>}
    </div>
  )
}

// TeamOvr — средний OVR команды перед матчем у её названия. Игроки в первом матче
// в среднее не входят: тогда рядом пометка «по N из M», а их ники — в подсказке.
export function TeamOvr({ team, newcomers, side }: { team?: TeamOVR; newcomers: string[]; side: 'A' | 'B' }) {
  if (!team) {
    const tip = 'Средний OVR команды не посчитан: у всех игроков команды это первый матч'
    return (
      <span title={tip} aria-label={tip} className="cursor-help text-fg-faint">
        —
      </span>
    )
  }
  const part = team.rated < team.total
  const tip =
    `Средний OVR команды перед матчем: ${team.score}` +
    (part && newcomers.length > 0
      ? ` — по ${team.rated} из ${team.total}: ${newcomers.join(', ')} ${newcomers.length > 1 ? 'играют' : 'играет'} первый матч`
      : '')
  const chip = part && <CoverageChip>по {team.rated} из {team.total}</CoverageChip>
  return (
    <span title={tip} className="inline-flex cursor-help items-center gap-1.5 whitespace-nowrap">
      {side === 'B' && chip}
      <ScoreBadge score={team.score} />
      {side === 'A' && chip}
    </span>
  )
}
