import { ChevronDown, Info, Swords } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { api, type MatchDuels, type PlayerDuels, type SessionDuels } from '../api'
import { plural } from '../format'
import { Alert } from './ui/Alert'
import { Badge } from './ui/Badge'
import { Button } from './ui/Button'
import { Card, CardFooter, CardHeader } from './ui/Card'
import { cx } from './ui/cx'
import { DuelLegend, DuelMatrix } from './ui/DuelMatrix'
import { EmptyState } from './ui/EmptyState'
import { Rivals } from './ui/Rivals'

type Load<T> = { data: T | null; error: string; reload: () => void }

// useDuels загружает дуэли отдельно от страницы. key описывает запрос: при его смене
// (другой период, изменились статусы матчей после пересчёта) данные запрашиваются снова.
function useDuels<T>(fetch: () => Promise<T>, key: string): Load<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  // fetch пересоздаётся при каждом рендере; запрос определяется ключом
  const fetchRef = useRef(fetch)
  useEffect(() => {
    fetchRef.current = fetch
  })
  useEffect(() => {
    let stale = false
    fetchRef.current().then(
      (d) => {
        if (stale) return
        setData(d)
        setError('')
      },
      (e: Error) => !stale && setError(e.message),
    )
    return () => {
      stale = true
    }
  }, [key, attempt])
  return { data, error, reload: () => setAttempt((a) => a + 1) }
}

// Заглушка и ошибка — общие для блоков дуэлей: блок падает отдельно от остальной страницы.
function DuelsLoading() {
  return (
    <div className="grid grid-cols-[120px_repeat(5,minmax(0,1fr))] items-center gap-x-3 gap-y-3.5 px-4 py-5 md:px-5" aria-busy>
      {Array.from({ length: 24 }, (_, i) => (
        <span
          key={i}
          className={cx('block animate-pulse rounded-[4px] bg-surface-2 motion-reduce:animate-none', i < 6 ? 'h-2.5' : i % 6 === 0 ? 'h-3' : 'h-[18px]')}
          style={i % 6 === 0 ? { width: `${[70, 90, 60, 80][i / 6]}%` } : undefined}
        />
      ))}
      <span className="sr-only" role="status">
        Загружаем дуэли
      </span>
    </div>
  )
}

function DuelsError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="px-4 py-5 md:px-5">
      <Alert
        tone="error"
        title="Не удалось загрузить дуэли."
        action={
          <Button size="sm" onClick={onRetry}>
            Повторить
          </Button>
        }
      >
        Сервер не ответил. Остальная статистика на странице актуальна.
      </Alert>
    </div>
  )
}

function NotComputed({ text, action }: { text: ReactNode; action?: ReactNode }) {
  return <EmptyState icon={<Swords />} title="Дуэли ещё не посчитаны" text={text} actions={action} className="py-7" />
}

const byMatches = (n: number) => `по ${plural(n, ['матчу', 'матчам', 'матчам'])}`
const ordinals = (list: { ordinal: number }[]) => list.map((m) => `#${m.ordinal}`).join(', ').replace(/, ([^,]*)$/, ' и $1')

function PartialBadge({ covered, eligible, title }: { covered: number; eligible: number; title?: string }) {
  return (
    <span title={title}>
      <Badge tone="neutral" icon={<Info aria-hidden />}>
        {byMatches(covered)} из {eligible}
      </Badge>
    </span>
  )
}

// MatchDuelsCard — матрица дуэлей матча: игроки по командам, как в scoreboard.
export function MatchDuelsCard({
  matchId,
  version,
  reparsing,
  onReparse,
}: {
  matchId: number
  // меняется вместе со статусом матча: после пересчёта дуэли запрашиваются снова
  version: string
  reparsing: boolean
  onReparse: () => Promise<void>
}) {
  const { data, error, reload } = useDuels<MatchDuels>(() => api.getMatchDuels(matchId), `${matchId}|${version}`)
  const [busy, setBusy] = useState(false)
  return (
    <Card aria-labelledby="duels-h">
      <CardHeader title="Дуэли" titleId="duels-h" note="строка — кто убивал, столбец — кого" />
      {error ? (
        <DuelsError onRetry={reload} />
      ) : !data ? (
        <DuelsLoading />
      ) : data.status === 'unavailable' ? (
        <NotComputed
          text="Матч обработан до появления дуэлей. Пересчитайте его — счёт и статистика не изменятся."
          action={
            <Button
              size="sm"
              loading={busy || reparsing}
              onClick={async () => {
                setBusy(true)
                await onReparse().finally(() => setBusy(false))
              }}
            >
              {reparsing ? 'Пересчитываем…' : 'Пересчитать матч'}
            </Button>
          }
        />
      ) : (
        <>
          <DuelMatrix players={data.players} cells={data.cells} context="match" labelledBy="duels-h" />
          <CardFooter>
            <DuelLegend allies="союзники" />
          </CardFooter>
        </>
      )}
    </Card>
  )
}

const COLLAPSE_KEY = 'cs2stats.sessionDuelsCollapsed'

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === '1'
  } catch {
    return false
  }
}

function writeCollapsed(v: boolean) {
  try {
    if (v) localStorage.setItem(COLLAPSE_KEY, '1')
    else localStorage.removeItem(COLLAPSE_KEY)
  } catch {
    /* хранилище недоступно — выбор живёт до перезагрузки */
  }
}

// SessionDuelsCard — матрица дуэлей вечера. Сворачивается; выбор общий для всех сессий.
export function SessionDuelsCard({
  sessionId,
  version,
  reparsing,
  onReparse,
}: {
  sessionId: string
  // меняется вместе со статусами матчей: после пересчёта дуэли запрашиваются снова
  version: string
  // идёт пересчёт матчей сессии: показаны прежние дуэли
  reparsing: boolean
  onReparse: (ids: number[]) => Promise<void>
}) {
  const { data, error, reload } = useDuels<SessionDuels>(() => api.getSessionDuels(sessionId), `${sessionId}|${version}`)
  const [collapsed, setCollapsed] = useState(readCollapsed)
  const [busy, setBusy] = useState(false)
  if (data?.status === 'no_matches') return null

  const toggle = () => {
    writeCollapsed(!collapsed)
    setCollapsed(!collapsed)
  }
  const uncovered = data?.uncoveredMatches ?? []
  const reparse = async () => {
    setBusy(true)
    await onReparse(uncovered.map((m) => m.id)).finally(() => setBusy(false))
  }
  const reparseButton = (label: string, link: boolean) =>
    link ? (
      <button
        type="button"
        className="focus-ring cursor-pointer rounded-[4px] border-0 bg-transparent p-0 font-[inherit] font-bold text-accent underline underline-offset-3 hover:text-accent-hover disabled:cursor-progress disabled:opacity-60"
        disabled={busy}
        onClick={reparse}
      >
        {label}
      </button>
    ) : (
      <Button size="sm" loading={busy} onClick={reparse}>
        {label}
      </Button>
    )
  const partial = data?.status === 'partial'
  const title = uncovered.length ? `Матчи ${ordinals(uncovered)} обработаны до появления дуэлей` : undefined

  return (
    <Card aria-labelledby="duels-h">
      <CardHeader
        title="Дуэли"
        titleId="duels-h"
        className={cx(collapsed && 'border-b-0')}
        note={
          data && data.status !== 'unavailable' ? (
            <span className="inline-flex flex-wrap items-center gap-3">
              {reparsing
                ? 'прежние данные — обновятся после пересчёта'
                : plural(data.players.length, ['игрок', 'игрока', 'игроков']) + (partial ? '' : ` · ${byMatches(data.coveredMatches)}`)}
              {partial && <PartialBadge covered={data.coveredMatches} eligible={data.eligibleMatches} title={title} />}
            </span>
          ) : undefined
        }
        aside={
          <Button
            size="sm"
            variant="ghost"
            aria-expanded={!collapsed}
            aria-controls="duels-body"
            leading={<ChevronDown className={cx('transition-transform', collapsed && '-rotate-90')} aria-hidden />}
            onClick={toggle}
          >
            {collapsed ? 'Показать матрицу' : 'Свернуть'}
          </Button>
        }
      />
      {!collapsed && (
        <div id="duels-body">
          {error ? (
            <DuelsError onRetry={reload} />
          ) : !data ? (
            <DuelsLoading />
          ) : data.status === 'unavailable' ? (
            <NotComputed
              text={`${uncovered.length === 1 ? 'Матч' : 'Матчи'} ${ordinals(uncovered)} обработаны до появления дуэлей. Пересчитайте их — счёт и статистика не изменятся.`}
              action={reparseButton('Пересчитать их', false)}
            />
          ) : (
            <>
              {partial && (
                <div className="border-b border-border px-4 py-2.5 text-[13px] leading-[19px] text-fg-muted md:px-5">
                  {uncovered.length === 1 ? 'Матч' : 'Матчи'} {ordinals(uncovered)} обработаны до появления дуэлей и в счёт не входят.{' '}
                  {reparseButton('Пересчитать их', true)}
                </div>
              )}
              <DuelMatrix players={data.players} cells={data.cells} context="session" labelledBy="duels-h" />
              <CardFooter>
                <DuelLegend allies="весь вечер в одной команде" />
              </CardFooter>
            </>
          )}
        </div>
      )}
    </Card>
  )
}

// RivalsCard — «Соперники» в профиле: чаще всего убивает и чаще всего погибает от.
export function RivalsCard({ steamId, name, query }: { steamId: string; name: string; query: string }) {
  const { data, error, reload } = useDuels<PlayerDuels>(() => api.getPlayerDuels(steamId, query), `${steamId}|${query}`)
  const partial = data?.status === 'partial'
  let body: ReactNode
  if (error) body = <DuelsError onRetry={reload} />
  else if (!data) body = <DuelsLoading />
  else if (data.status === 'no_matches') body = <Empty>Нет игр за выбранный период. Измените даты или выберите другие сессии.</Empty>
  else if (data.status === 'unavailable')
    body = <Empty>Дуэли ещё не посчитаны: матчи за этот период обработаны старой версией. Пересчитайте их на страницах сессий.</Empty>
  else if (data.mostKilled.length === 0 && data.mostKilledBy.length === 0)
    body = <Empty>Личных убийств нет: за выбранный период {name} ни разу не убил соперника и не погиб от него.</Empty>
  else
    body = (
      <Rivals
        groups={[
          { title: 'Чаще всего убивает', list: data.mostKilled, by: 'kills' },
          { title: 'Чаще всего погибает от', list: data.mostKilledBy, by: 'deaths' },
        ]}
      />
    )
  return (
    <Card aria-labelledby="rivals-h">
      <CardHeader
        title="Соперники"
        titleId="rivals-h"
        note={
          <span className="inline-flex flex-wrap items-center gap-3">
            счёт «убил : погиб» — со стороны {name}
            {partial && <PartialBadge covered={data.coveredMatches} eligible={data.eligibleMatches} />}
          </span>
        }
      />
      {body}
    </Card>
  )
}

function Empty({ children }: { children: ReactNode }) {
  return <p className="m-0 px-4 py-5 text-[13px] leading-[19px] text-fg-muted md:px-5">{children}</p>
}
