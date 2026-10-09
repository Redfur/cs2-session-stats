import { Link } from 'react-router'
import type { Opponent } from '../../api'
import { duelPct } from '../../duels'
import { plural } from '../../format'
import { cx } from './cx'

interface RivalGroup {
  title: string
  list: Opponent[]
  // какое число в счёте значимо для показателя: убийства или смерти
  by: 'kills' | 'deaths'
  // текст, если подходящих соперников нет
  empty: string
}

const TIES: Record<number, string> = { 2: 'двое поровну', 3: 'трое поровну' }

const cardsText = (n: number) => `на ${plural(n, ['карте', 'картах', 'картах'])}`

// Rivals — два показателя профиля: соперники с наибольшей и наименьшей долей дуэлей.
// Счёт «убил : погиб» и доля — со стороны владельца профиля.
export function Rivals({ groups }: { groups: RivalGroup[] }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2">
      {groups.map((g, gi) => (
        <div
          key={g.title}
          className={cx(
            'flex min-w-0 flex-col gap-2 px-4 py-3.5 md:px-5 md:py-4',
            gi > 0 && 'border-t border-border md:border-t-0 md:border-l',
          )}
        >
          <div className="flex min-h-4 items-baseline justify-between gap-2">
            <h3 className="m-0 text-over font-bold uppercase text-fg-muted">{g.title}</h3>
            {g.list.length > 1 && <span className="text-small text-fg-muted">{TIES[g.list.length] ?? `${g.list.length} поровну`}</span>}
          </div>
          {g.list.length === 0 && <p className="m-0 py-[7px] text-[13px] leading-[19px] text-fg-muted">{g.empty}</p>}
          <ul className="m-0 flex list-none flex-col p-0">
            {g.list.map((o) => (
              <li
                key={o.steamId}
                className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-x-3 gap-y-0.5 border-t border-border py-[7px] first:border-t-0 md:grid-cols-[minmax(0,1fr)_auto_88px]"
              >
                <Link to={`/players/${o.steamId}`} title={o.name} className="focus-ring block min-w-0 truncate font-semibold text-fg hover:text-accent-hover">
                  {o.name}
                </Link>
                <span
                  className="inline-flex items-baseline gap-1.5 text-[15px] leading-5 tabular-nums"
                  aria-label={`убил ${o.kills}, погиб ${o.deaths}, доля ${duelPct(o.kills, o.deaths)}%, ${cardsText(o.maps)}`}
                >
                  <span className={g.by === 'kills' ? 'font-extrabold text-fg' : 'font-medium text-fg-muted'}>{o.kills}</span>
                  <span className="font-medium text-fg-faint">:</span>
                  <span className={g.by === 'deaths' ? 'font-extrabold text-fg' : 'font-medium text-fg-muted'}>{o.deaths}</span>
                  <span className="ml-1 text-small font-semibold text-fg-muted">{duelPct(o.kills, o.deaths)}%</span>
                </span>
                <span className="col-span-full text-small whitespace-nowrap text-fg-muted md:col-span-1 md:text-right" aria-hidden>
                  {cardsText(o.maps)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  )
}
