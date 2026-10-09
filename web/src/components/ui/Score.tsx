import { cx } from './cx'

const LOSE = 'opacity-55 font-medium'

interface ScoreProps {
  a: number
  b: number
  // teams — цвета команд A и B; own — своя команда слева, соперник приглушён
  kind?: 'teams' | 'own'
  className?: string
}

// Score — счёт матча; проигравшая сторона приглушена.
export function Score({ a, b, kind = 'teams', className }: ScoreProps) {
  return (
    <span className={cx('inline-flex items-baseline gap-[5px] whitespace-nowrap font-bold tabular-nums', className)}>
      <span className={cx(kind === 'teams' ? 'text-team-a' : 'text-fg', a < b && LOSE)}>{a}</span>
      <span className="font-medium text-fg-faint">:</span>
      <span className={cx(kind === 'teams' ? 'text-team-b' : 'text-fg-muted', b < a && LOSE)}>{b}</span>
    </span>
  )
}

export function TeamName({ team, className }: { team: 'A' | 'B'; className?: string }) {
  const sw = <span className={cx('size-2.5 flex-none rounded-[3px]', team === 'A' ? 'bg-team-a' : 'bg-team-b')} aria-hidden />
  return (
    <span className={cx('inline-flex items-center gap-2 whitespace-nowrap font-bold', className)}>
      {team === 'A' && sw}
      Команда {team}
      {team === 'B' && sw}
    </span>
  )
}
