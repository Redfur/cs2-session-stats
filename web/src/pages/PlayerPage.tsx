import { ExternalLink } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useParams, useSearchParams } from 'react-router'
import { api, sessionTitle, type PlayerProfile, type ProfileMap, type ProfileMatch, type ProfileSession } from '../api'
import { RivalsCard } from '../components/Duels'
import { FightTab, UtilityTab, WeaponsTab } from '../components/PlayerExtTabs'
import { PageError, PageLoading } from '../components/PageState'
import { PeriodBar } from '../components/PeriodFilter'
import { PLAYER_TOTAL_COLUMNS, PLAYER_TOTAL_MIN_WIDTH, RATING_DESC } from '../components/playerColumns'
import { Alert } from '../components/ui/Alert'
import { OutcomeBadge } from '../components/ui/Badge'
import { Card, CardBody, CardHeader } from '../components/ui/Card'
import { Page } from '../components/ui/Layout'
import { MetaItem, PageHeader } from '../components/ui/PageHeader'
import { Score } from '../components/ui/Score'
import { Tabs } from '../components/ui/Tabs'
import { rowLinkClass, StatTable, type Column } from '../components/ui/StatTable'
import { StatTile } from '../components/ui/StatTile'
import { formatDateLong, formatDateShort, plural } from '../format'
import { formatMetric, formatPct } from '../metrics'
import { usePeriod, useSessionList } from '../period'
import { sortRows, useSort, type SortState } from '../sort'
import { NotFoundPage } from './NotFoundPage'

// Параметры периода в адресе страницы: переносятся между вкладками профиля и уходят в запросы.
const PERIOD_PARAMS = ['from', 'to', 'session', 'period']

// Вкладки профиля с собственными адресами; «Обзор» — без суффикса.
const TABS = [
  { tab: undefined, path: '', label: 'Обзор' },
  { tab: 'fight', path: '/fight', label: 'Бой' },
  { tab: 'weapons', path: '/weapons', label: 'Оружие' },
  { tab: 'utility', path: '/utility', label: 'Гранаты и бомба' },
]

const mp = { key: 'm', label: 'М', title: 'Матчи', numeric: true, secondary: true, width: 36 } as const
const wp = { key: 'w', label: 'П', title: 'Победы', numeric: true, secondary: true, width: 36 } as const

const SESSION_COLUMNS: Column<ProfileSession>[] = [
  {
    key: 'session',
    label: 'Сессия',
    value: (s) => sessionTitle(s.session),
    render: (s) => (
      <Link to={`/sessions/${s.session.id}`} className={rowLinkClass}>
        {sessionTitle(s.session)}
      </Link>
    ),
  },
  { key: 'date', label: 'Дата', numeric: true, secondary: true, width: 96, value: (s) => s.session.date, render: (s) => formatDateShort(s.session.date) },
  { ...mp, value: (s) => s.matches ?? 0 },
  { ...wp, value: (s) => s.wins ?? 0 },
  { key: 'kd', label: 'K/D', numeric: true, metric: 'kd', width: 60, value: (s) => s.kd },
  { key: 'adr', label: 'ADR', numeric: true, metric: 'adr', width: 66, value: (s) => s.adr },
  { key: 'kast', label: 'KAST', numeric: true, metric: 'kast', width: 60, value: (s) => s.kastPct },
  { key: 'rating', label: 'Rating', numeric: true, metric: 'rating', strong: true, width: 70, value: (s) => s.rating },
]
const SESSIONS_DEFAULT: SortState = { key: 'date', dir: 'desc' }

const MAP_COLUMNS: Column<ProfileMap>[] = [
  { key: 'map', label: 'Карта', value: (m) => m.map || '—', render: (m) => <span className="font-semibold">{m.map || '—'}</span> },
  { ...mp, value: (m) => m.matches ?? 0 },
  { ...wp, value: (m) => m.wins ?? 0 },
  { key: 'kd', label: 'K/D', numeric: true, metric: 'kd', width: 60, value: (m) => m.kd },
  { key: 'adr', label: 'ADR', numeric: true, metric: 'adr', width: 66, value: (m) => m.adr },
  { key: 'rating', label: 'Rating', numeric: true, metric: 'rating', strong: true, width: 70, value: (m) => m.rating },
]

// счёт со стороны игрока: его команда слева
const ownScore = (m: ProfileMatch) => (m.team === 'B' ? [m.scoreB, m.scoreA] : [m.scoreA, m.scoreB])
const RESULT_ORDER = { win: 2, draw: 1, loss: 0 } as const

const MATCH_COLUMNS: Column<ProfileMatch>[] = [
  {
    key: 'session',
    label: 'Сессия',
    value: (m) => m.sessionTitle || formatDateLong(m.sessionDate),
    render: (m) => (
      <Link to={`/sessions/${m.sessionId}`} className={rowLinkClass}>
        {m.sessionTitle || formatDateLong(m.sessionDate)}
      </Link>
    ),
  },
  {
    // порядок матчей во времени: дата сессии, сессия, номер матча
    key: 'n',
    label: '#',
    name: 'по времени',
    numeric: true,
    width: 44,
    value: (m) => `${m.sessionDate}|${String(m.sessionId).padStart(10, '0')}|${String(m.ordinal).padStart(4, '0')}`,
    render: (m) => (
      <Link to={`/matches/${m.matchId}`} className="focus-ring font-bold">
        {m.ordinal}
      </Link>
    ),
  },
  {
    key: 'map',
    label: 'Карта',
    width: 130,
    value: (m) => m.map,
    render: (m) => (
      <Link to={`/matches/${m.matchId}`} className={rowLinkClass}>
        {m.map || '—'}
      </Link>
    ),
  },
  {
    key: 'score',
    label: 'Счёт',
    numeric: true,
    width: 84,
    value: (m) => ownScore(m)[0] - ownScore(m)[1],
    render: (m) => {
      const [own, other] = ownScore(m)
      return <Score a={own} b={other} kind="own" />
    },
  },
  {
    key: 'result',
    label: 'Исход',
    align: 'left',
    numeric: true,
    width: 110,
    value: (m) => (m.result ? RESULT_ORDER[m.result] : -1),
    render: (m) => m.result && <OutcomeBadge result={m.result} />,
  },
  {
    key: 'kda',
    label: 'K-D-A',
    numeric: true,
    secondary: true,
    width: 100,
    value: (m) => m.kills,
    render: (m) => `${m.kills}-${m.deaths}-${m.assists}`,
  },
  { key: 'adr', label: 'ADR', numeric: true, metric: 'adr', width: 72, value: (m) => m.adr },
  { key: 'rating', label: 'Rating', numeric: true, metric: 'rating', strong: true, width: 76, value: (m) => m.rating },
]
const MATCHES_DEFAULT: SortState = { key: 'n', dir: 'desc' }

export function PlayerPage() {
  const { steamId = '', tab } = useParams()
  const [params, setParams] = useSearchParams()
  const [data, setData] = useState<PlayerProfile | null>(null)
  const [error, setError] = useState('')
  const sessions = useSessionList()
  const period = usePeriod(params, setParams)
  const ss = useSort('ss', SESSION_COLUMNS, SESSIONS_DEFAULT)
  const sm = useSort('sm', MAP_COLUMNS, RATING_DESC)
  const sx = useSort('sx', MATCH_COLUMNS, MATCHES_DEFAULT)

  // в запрос идёт только период: сортировки таблиц остаются на клиенте
  const query = useMemo(() => {
    const q = new URLSearchParams()
    for (const k of PERIOD_PARAMS) for (const v of params.getAll(k)) q.append(k, v)
    return q.toString()
  }, [params])

  const load = useCallback(() => {
    let stale = false
    api.getPlayer(steamId, query).then(
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
  }, [steamId, query])

  useEffect(load, [load])

  if (!TABS.some((t) => t.tab === tab)) return <NotFoundPage />
  if (error && !data)
    return <PageError title="Не удалось открыть профиль игрока" error={error} onRetry={load} back={{ to: '/players', label: 'Все игроки' }} />
  if (!data) return <PageLoading />

  const t = data.totals
  const matches = t.matches ?? 0
  const wins = t.wins ?? 0

  return (
    <Page>
      <PageHeader
        back={{ to: '/players', label: 'Все игроки' }}
        title={data.name}
        meta={
          <>
            <a
              className="focus-ring inline-flex items-center gap-1.5"
              href={`https://steamcommunity.com/profiles/${data.steamId}`}
              target="_blank"
              rel="noreferrer"
            >
              Профиль Steam
              <ExternalLink className="size-3.5" aria-hidden />
            </a>
            <MetaItem className="tabular-nums">
              {plural(matches, ['матч', 'матча', 'матчей'])} · {plural(data.sessions.length, ['сессия', 'сессии', 'сессий'])}
            </MetaItem>
          </>
        }
        actions={<PeriodBar period={period} sessions={sessions} />}
      />

      {error && (
        <Alert tone="error" title="Фильтр не применён.">
          Ответ сервера: {error}
        </Alert>
      )}

      <div className="grid grid-cols-2 gap-2.5 md:grid-cols-6 md:gap-4">
        <StatTile label="Rating" value={formatMetric('rating', t.rating)} sub="HLTV 1.0" metric={{ metric: 'rating', value: t.rating }} />
        <StatTile label="ADR" value={formatMetric('adr', t.adr)} sub="урон за раунд" metric={{ metric: 'adr', value: t.adr }} />
        <StatTile label="K/D" value={formatMetric('kd', t.kd)} sub={`${t.kills} / ${t.deaths}`} metric={{ metric: 'kd', value: t.kd }} />
        <StatTile label="KAST" value={formatMetric('kast', t.kastPct)} sub="полезные раунды" metric={{ metric: 'kast', value: t.kastPct }} />
        <StatTile label="HS%" value={formatPct(t.hsPct)} sub="в голову" />
        <StatTile
          label="Победы"
          value={
            <>
              {wins}
              <span className="font-semibold text-fg-faint"> / {matches}</span>
            </>
          }
          sub={matches ? `${Math.round((wins / matches) * 100)}% матчей` : 'нет матчей'}
        />
      </div>

      <Tabs
        label="Разделы профиля"
        items={TABS.map((t) => ({
          to: `/players/${steamId}${t.path}${query ? `?${query}` : ''}`,
          label: t.label,
          end: t.tab === undefined,
        }))}
      />

      {tab === 'fight' && <FightTab steamId={steamId} query={query} sessions={sessions} />}
      {tab === 'weapons' && <WeaponsTab steamId={steamId} query={query} sessions={sessions} />}
      {tab === 'utility' && <UtilityTab steamId={steamId} query={query} sessions={sessions} />}
      {tab === undefined && <Overview data={data} steamId={steamId} query={query} ss={ss} sm={sm} sx={sx} />}
    </Page>
  )
}

interface OverviewProps {
  data: PlayerProfile
  steamId: string
  query: string
  ss: { sort: SortState; toggle: (key: string) => void }
  sm: { sort: SortState; toggle: (key: string) => void }
  sx: { sort: SortState; toggle: (key: string) => void }
}

// Overview — вкладка «Обзор»: итоги, разбивки, соперники и матчи.
function Overview({ data, steamId, query, ss, sm, sx }: OverviewProps) {
  const t = data.totals
  return (
    <>
      <Card aria-labelledby="sum-h">
        <CardHeader title="Итоги" titleId="sum-h" note="все колонки за выбранный период" />
        <StatTable
          columns={PLAYER_TOTAL_COLUMNS}
          rows={[t]}
          rowKey={(p) => p.steamId}
          highlight={() => true}
          labelledBy="sum-h"
          minWidth={PLAYER_TOTAL_MIN_WIDTH}
        />
      </Card>

      <div className="grid grid-cols-1 items-start gap-6 md:grid-cols-2">
        <Breakdown
          id="ps-h"
          title="По сессиям"
          columns={SESSION_COLUMNS}
          rows={data.sessions}
          rowKey={(s) => s.session.id}
          sort={ss}
          minWidth={580}
        />
        <Breakdown id="pm-h" title="По картам" columns={MAP_COLUMNS} rows={data.maps} rowKey={(m) => m.map} sort={sm} minWidth={380} />
      </div>

      <RivalsCard steamId={steamId} name={data.name} query={query} />

      <Breakdown
        id="mm-h"
        title="Матчи"
        columns={MATCH_COLUMNS}
        rows={data.matches}
        rowKey={(m) => m.matchId}
        sort={sx}
        minWidth={780}
        aside={<span className="text-small text-fg-muted">Счёт — со стороны игрока: его команда слева</span>}
      />
    </>
  )
}

interface BreakdownProps<T> {
  id: string
  title: string
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string | number
  sort: { sort: SortState; toggle: (key: string) => void }
  minWidth: number
  aside?: ReactNode
}

// Breakdown — разбивка профиля со своей сортировкой в адресе страницы.
function Breakdown<T>({ id, title, columns, rows, rowKey, sort, minWidth, aside }: BreakdownProps<T>) {
  return (
    <Card aria-labelledby={id}>
      <CardHeader title={title} titleId={id} note={String(rows.length)} aside={aside} />
      {rows.length > 0 ? (
        <StatTable
          columns={columns}
          rows={sortRows(rows, columns, sort.sort)}
          rowKey={rowKey}
          sort={sort.sort}
          onSort={sort.toggle}
          labelledBy={id}
          minWidth={minWidth}
        />
      ) : (
        <CardBody>
          <p className="m-0 text-[13px] text-fg-muted">Нет матчей за выбранный период.</p>
        </CardBody>
      )}
    </Card>
  )
}
