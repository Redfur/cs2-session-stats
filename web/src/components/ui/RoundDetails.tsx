import type { Round } from '../../api'
import { mmss, REASON_TEXT, sideOf } from '../../rounds'
import { Badge } from './Badge'
import { cx } from './cx'
import { SideChip, SideSwatch } from './SideChip'
import { WeaponIcon } from './WeaponIcon'

export interface RoundPlayer {
  name: string
  team: 'A' | 'B'
}

const OUTCOME = { win: 'выиграл', loss: 'проиграл', draw: 'ничья', unknown: 'исход неизвестен' } as const

function Team({ team, side, small }: { team: 'A' | 'B'; side?: ReturnType<typeof sideOf>; small?: boolean }) {
  return (
    <span className={cx('inline-flex items-center gap-2 font-bold whitespace-nowrap', small && 'text-small')}>
      <span className={cx('size-2.5 flex-none rounded-[3px]', team === 'A' ? 'bg-team-a' : 'bg-team-b')} aria-hidden />
      Команда {team}
      {side && <SideChip side={side} />}
    </span>
  )
}

interface RoundDetailsProps {
  round: Round
  players: Map<string, RoundPlayer>
  id?: string
}

// RoundDetails — события выбранного раунда: шапка, ход раунда и кто дожил до конца.
export function RoundDetails({ round: r, players, id }: RoundDetailsProps) {
  const name = (steamId: string) => (steamId === '0' ? 'Мир' : (players.get(steamId)?.name ?? steamId))
  const side = (steamId: string) => {
    const p = players.get(steamId)
    return p ? sideOf(r, p.team) : undefined
  }
  return (
    <div id={id} aria-live="polite" className="border-t border-border">
      <div className="flex flex-wrap items-center gap-x-3.5 gap-y-2 border-b border-border bg-surface-2 px-4 py-3 md:px-5">
        <span className="text-sub">Раунд {r.number}</span>
        <Team team={r.winner} side={r.restored ? undefined : sideOf(r, r.winner)} />
        {!r.restored && (
          <span className="text-small text-fg-muted">
            {REASON_TEXT[r.reason]}
            {r.durationSec != null && ` · ${mmss(r.durationSec)}`}
          </span>
        )}
        <span className="text-small text-fg-muted tabular-nums">
          счёт после раунда {r.scoreA} : {r.scoreB}
        </span>
        {r.clutches.map((c) => (
          <Badge key={c.steamId} tone="neutral">
            Клатч 1v{c.vs} · {name(c.steamId)} — {OUTCOME[c.outcome]}
          </Badge>
        ))}
        {r.restored && <Badge tone="neutral">восстановлен</Badge>}
      </div>
      {r.restored ? (
        <p className="m-0 px-4 py-4 text-small text-fg-muted md:px-5">
          Демка оборвалась до конца этого раунда. Победитель восстановлен из итогового счёта матча; время, причина и ход
          раунда неизвестны.
        </p>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)]">
          <div className="flex min-w-0 flex-col gap-2 px-4 pt-3 pb-3.5 md:px-5 md:pt-3.5 md:pb-4">
            <span className="text-over font-bold text-fg-muted uppercase">Ход раунда</span>
            {r.kills.length === 0 ? (
              <p className="m-0 text-small text-fg-muted">Убийств не было.</p>
            ) : (
              <ol className="m-0 flex list-none flex-col p-0">
                {r.kills.map((k, i) => {
                  const tags = [
                    k.headshot && 'HS',
                    k.trade && 'размен',
                    k.throughSmoke && 'через дым',
                    k.wallbang && 'прострел',
                  ].filter(Boolean) as string[]
                  const ks = side(k.killerId)
                  const vs = side(k.victimId)
                  return (
                    <li
                      key={i}
                      className="grid min-h-9 grid-cols-[36px_minmax(0,1fr)] items-center gap-x-3 gap-y-1 border-t border-border py-1.5 first:border-t-0 md:grid-cols-[40px_minmax(0,1fr)_auto]"
                    >
                      <span className="text-small text-fg-faint tabular-nums">{k.timeSec != null ? mmss(k.timeSec) : '—'}</span>
                      <span className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-1">
                        <span className="inline-flex items-center gap-1.5 font-semibold whitespace-nowrap">
                          {ks && <SideSwatch side={ks} />}
                          {name(k.killerId)}
                        </span>
                        <WeaponIcon code={k.weapon} className="text-metric-mid" />
                        <span className="inline-flex items-center gap-1.5 font-medium whitespace-nowrap text-fg-muted">
                          {vs && <SideSwatch side={vs} />}
                          {name(k.victimId)}
                        </span>
                      </span>
                      {tags.length > 0 && (
                        <span className="col-start-2 flex flex-wrap gap-1 md:col-start-3 md:justify-end">
                          {tags.map((t) => (
                            <span
                              key={t}
                              className="inline-flex h-5 items-center rounded-[4px] bg-surface-2 px-1.5 text-over font-semibold whitespace-nowrap text-fg-muted shadow-[inset_0_0_0_1px_var(--color-border)]"
                            >
                              {t}
                            </span>
                          ))}
                        </span>
                      )}
                    </li>
                  )
                })}
              </ol>
            )}
          </div>
          <div className="flex min-w-0 flex-col gap-2 border-t border-border px-4 pt-3 pb-3.5 md:border-t-0 md:border-l md:px-5 md:pt-3.5 md:pb-4">
            <span className="text-over font-bold text-fg-muted uppercase">Дожили до конца</span>
            <div className="flex flex-col gap-2.5">
              {(['A', 'B'] as const).map((team) => {
                const names = (r.alive?.[team] ?? []).map(name)
                return (
                  <div key={team} className="flex flex-col gap-1">
                    <Team team={team} side={sideOf(r, team)} small />
                    <span className={names.length ? '' : 'text-fg-faint'}>{names.length ? names.join(', ') : 'никто'}</span>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
