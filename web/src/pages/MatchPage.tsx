import { ChevronLeft, ChevronRight, LoaderCircle, RefreshCw, Trash } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { Swords } from 'lucide-react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { api, isProcessing, sessionTitle, type MatchDetails, type MatchExt, type PlayerRow, type SessionDetails, type Side } from '../api'
import { detailColumns, joinDetails, type DetailPlayer } from '../components/detailColumns'
import { MatchDuelsCard } from '../components/Duels'
import { useExtLoad } from '../ext'
import { MatchQualityAlert, MatchRounds, MatchWeaponsCard, TeamSides } from '../components/MatchExt'
import { isIncomplete, matchDuration } from '../rounds'
import { PageError, PageLoading } from '../components/PageState'
import { PLAYER_MATCH_COLUMNS, PLAYER_MATCH_MIN_WIDTH, RATING_DESC } from '../components/playerColumns'
import { Alert } from '../components/ui/Alert'
import { Badge, OutcomeBadge, StatusBadge } from '../components/ui/Badge'
import { Button, ButtonLink } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { cx } from '../components/ui/cx'
import { ConfirmDialog } from '../components/ui/Dialog'
import { EmptyState } from '../components/ui/EmptyState'
import { Page } from '../components/ui/Layout'
import { MetaItem, PageHeader } from '../components/ui/PageHeader'
import { TeamName } from '../components/ui/Score'
import { ScoreBoard } from '../components/ui/ScoreBoard'
import { Segment } from '../components/ui/Segment'
import { StatTable, type Column } from '../components/ui/StatTable'
import { plural } from '../format'
import { sortRows, useSort, type SortState } from '../sort'

const POLL_MS = 3000

const DETAIL_COLUMNS = detailColumns('match', {
  old: 'Матч обработан старой версией — пересчитайте его',
  noDamage: 'В демке нет событий урона — не посчитано',
  noFlash: 'В демке нет событий ослепления — не посчитано',
})
const ADR_DESC: SortState = { key: 'adr', dir: 'desc' }

type View = 'basic' | 'detail'

export function MatchPage() {
  const { id = '' } = useParams()
  const [data, setData] = useState<MatchDetails | null>(null)
  // сессия матча: название и соседние матчи
  const [session, setSession] = useState<SessionDetails | null>(null)
  const [error, setError] = useState('')
  const [confirm, setConfirm] = useState<'delete' | 'reparse' | null>(null)
  const navigate = useNavigate()

  const load = useCallback(() => {
    api.getMatch(id).then(
      (d) => {
        setData(d)
        setError('')
      },
      (e: Error) => setError(e.message),
    )
  }, [id])

  useEffect(load, [load])

  const sessionId = data?.match.sessionId
  useEffect(() => {
    if (sessionId === undefined) return
    api.getSession(String(sessionId)).then(setSession, () => setSession(null))
  }, [sessionId])

  // пока матч в очереди или в обработке — опрашиваем сервер
  const processing = data ? isProcessing(data.match) : false
  useEffect(() => {
    if (!processing) return
    const t = setInterval(load, POLL_MS)
    return () => clearInterval(t)
  }, [processing, load])

  const { sort, toggle } = useSort('sort', PLAYER_MATCH_COLUMNS, RATING_DESC)
  const detailSort = useSort('sort', DETAIL_COLUMNS, ADR_DESC)
  const [params, setParams] = useSearchParams()
  const view: View = params.get('view') === 'detail' ? 'detail' : 'basic'
  const setView = (v: View) =>
    setParams(
      (prev) => {
        const p = new URLSearchParams(prev)
        if (v === 'detail') p.set('view', 'detail')
        else p.delete('view')
        p.delete('sort') // у видов разные колонки
        return p
      },
      { replace: true },
    )

  // меняется вместе со статусом матча: после пересчёта данные запрашиваются снова
  const version = data ? `${data.match.status}|${data.match.processedVersion}|${data.match.parsedAt ?? ''}` : ''
  const extLoad = useExtLoad<MatchExt>(() => api.getMatchExt(id), `${id}|${version}`)
  const ext = extLoad.data?.status === 'complete' ? extLoad.data : null

  if (error && !data)
    return <PageError title="Не удалось открыть матч" error={error} onRetry={load} back={{ to: '/', label: 'К списку сессий' }} />
  if (!data) return <PageLoading />

  const { match, players } = data
  const siblings = session && session.session.id === match.sessionId ? session.matches : null
  const index = siblings?.findIndex((m) => m.id === match.id) ?? -1
  const prev = index > 0 ? siblings![index - 1] : null
  const next = index >= 0 && siblings && index < siblings.length - 1 ? siblings[index + 1] : null

  const nav = siblings && index >= 0 && (
    <div className="mr-2 flex items-center gap-1">
      {prev ? (
        <ButtonLink to={`/matches/${prev.id}`} icon aria-label={`Предыдущий матч #${index}`}>
          <ChevronLeft aria-hidden />
        </ButtonLink>
      ) : (
        <Button icon disabled aria-label="Предыдущий матч">
          <ChevronLeft aria-hidden />
        </Button>
      )}
      <span className="min-w-[52px] text-center text-small text-fg-muted tabular-nums">
        {index + 1} из {siblings.length}
      </span>
      {next ? (
        <ButtonLink to={`/matches/${next.id}`} icon aria-label={`Следующий матч #${index + 2}`}>
          <ChevronRight aria-hidden />
        </ButtonLink>
      ) : (
        <Button icon disabled aria-label="Следующий матч">
          <ChevronRight aria-hidden />
        </Button>
      )}
    </div>
  )

  return (
    <Page>
      <PageHeader
        back={{ to: `/sessions/${match.sessionId}`, label: session ? `к сессии ${sessionTitle(session.session)}` : 'к сессии' }}
        title={
          <>
            Матч #{match.ordinal}
            {match.map && (
              <>
                {' '}
                <span className="font-medium text-fg-faint">—</span> {match.map}
              </>
            )}
          </>
        }
        meta={
          <>
            <StatusBadge match={match} showError={false} />
            {isIncomplete(ext, match) && (
              <span title="Запись демки неполная — подробности под заголовком">
                <Badge tone="neutral">Неполная запись</Badge>
              </span>
            )}
            <MetaItem className="min-w-0 overflow-hidden font-mono text-small text-ellipsis">{match.originalName}</MetaItem>
          </>
        }
        actions={
          <>
            {nav}
            <Button leading={<RefreshCw aria-hidden />} disabled={processing} onClick={() => setConfirm('reparse')}>
              Пересчитать
            </Button>
            <Button variant="danger-quiet" icon aria-label="Удалить матч" title="Удалить матч" onClick={() => setConfirm('delete')}>
              <Trash aria-hidden />
            </Button>
          </>
        }
      />

      {error && (
        <Alert tone="error" title="Не удалось обновить данные матча." action={<Button size="sm" onClick={load}>Повторить</Button>}>
          {error}
        </Alert>
      )}
      {match.status === 'failed' &&
        (match.hasResult ? (
          <Alert tone="error" title="Пересчёт не удался, показаны прежние данные.">
            {match.error}
          </Alert>
        ) : (
          <Alert tone="error" title="Ошибка обработки.">
            {match.error}
          </Alert>
        ))}
      {processing &&
        (match.hasResult ? (
          <Alert tone="info" title="Идёт пересчёт.">
            Пока показываем прежние счёт и статистику, страница обновится сама.
          </Alert>
        ) : (
          <Alert tone="info" icon={<LoaderCircle className="animate-spin" aria-hidden />} title="Демка обрабатывается.">
            Счёт и статистика появятся, когда обработка закончится.
          </Alert>
        ))}

      {ext && <MatchQualityAlert ext={ext} match={match} />}

      {match.hasResult && (
        <>
          <ScoreBoard
            scoreA={match.scoreA}
            scoreB={match.scoreB}
            duration={matchDuration(ext)}
            footer={ext ? undefined : `${plural(match.rounds, ['раунд', 'раунда', 'раундов'])} · Команда A начала матч за CT`}
          >
            {ext && <MatchRounds ext={ext} players={players} />}
          </ScoreBoard>

          <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <Segment
              label="Вид таблиц команд"
              value={view}
              onChange={setView}
              options={[
                { value: 'basic', label: 'Основное' },
                { value: 'detail', label: 'Подробно' },
              ]}
            />
            <span className="text-small text-fg-muted">
              {view === 'basic'
                ? 'K, D, A, ADR, KAST и рейтинг'
                : ext?.quality.restoredRound
                  ? `Подробно — по ${ext.extRounds} раундам из ${match.rounds}: последний раунд восстановлен из счёта`
                  : 'Размены, клатчи, урон гранат, полученный урон и выживание'}
            </span>
          </div>

          {view === 'detail' && extLoad.data && !ext ? (
            <Card as="div">
              <EmptyState
                icon={<Swords />}
                as="h2"
                title="Подробная статистика ещё не посчитана"
                text="Матч обработан до появления подробной статистики. Пересчитайте его — счёт и основные показатели не изменятся."
                actions={
                  <Button size="sm" loading={processing} onClick={() => setConfirm('reparse')}>
                    {processing ? 'Пересчитываем…' : 'Пересчитать матч'}
                  </Button>
                }
              />
            </Card>
          ) : (
            (['A', 'B'] as const).map((team) => (
              <TeamCard
                key={team}
                team={team}
                own={team === 'A' ? match.scoreA : match.scoreB}
                other={team === 'A' ? match.scoreB : match.scoreA}
                players={players}
                firstSide={ext && ext.rounds.length > 0 ? (team === 'A' ? ext.rounds[0].sideA : ext.rounds[0].sideA === 'CT' ? 'T' : 'CT') : undefined}
                {...(view === 'detail'
                  ? { detail: joinDetails(players, ext?.players), sort: detailSort.sort, onSort: detailSort.toggle }
                  : { sort, onSort: toggle })}
              />
            ))
          )}
          {ext && <MatchWeaponsCard ext={ext} players={players} />}
          <MatchDuelsCard
            matchId={match.id}
            version={`${match.status}|${match.processedVersion}|${match.parsedAt ?? ''}`}
            reparsing={processing}
            onReparse={async () => {
              const m = await api.reparseMatch(id)
              setData((d) => (d ? { ...d, match: m } : d))
            }}
          />
        </>
      )}

      <ConfirmDialog
        open={confirm === 'reparse'}
        onClose={() => setConfirm(null)}
        tone="normal"
        title="Пересчитать матч?"
        description={
          <>
            Матч <b>#{match.ordinal}</b> снова попадёт в очередь на разбор. Пока идёт пересчёт, на странице остаются прежние счёт и
            статистика.
          </>
        }
        confirmLabel="Пересчитать"
        errorTitle="Не удалось поставить пересчёт."
        onConfirm={async () => {
          const m = await api.reparseMatch(id)
          setData((d) => (d ? { ...d, match: m } : d))
        }}
      />
      <ConfirmDialog
        open={confirm === 'delete'}
        onClose={() => setConfirm(null)}
        tone="danger"
        title="Удалить матч?"
        description={
          <>
            Матч{' '}
            <b>
              #{match.ordinal}
              {match.map && ` — ${match.map}`}
            </b>{' '}
            будет удалён вместе с демкой и статистикой. Отменить это нельзя.
          </>
        }
        confirmLabel="Удалить матч"
        errorTitle="Не удалось удалить матч."
        onConfirm={async () => {
          await api.deleteMatch(match.id)
          navigate(`/sessions/${match.sessionId}`)
        }}
      />
    </Page>
  )
}

interface TeamCardProps {
  team: 'A' | 'B'
  own: number
  other: number
  players: PlayerRow[]
  // сторона команды в первой половине: в шапке видно CT → T
  firstSide?: Side
  // подробные строки игроков: таблица «Подробно»
  detail?: DetailPlayer[]
  sort: SortState
  onSort: (key: string) => void
}

// TeamCard — шапка команды с исходом и суммами K·D·A и таблица её игроков.
function TeamCard({ team, own, other, players, firstSide, detail, sort, onSort }: TeamCardProps) {
  const rows = players.filter((p) => p.team === team)
  const sum = (f: (p: PlayerRow) => number) => rows.reduce((s, p) => s + f(p), 0)
  const headId = `team-${team}`
  return (
    <Card aria-labelledby={headId}>
      <div
        className={cx(
          'flex min-h-14 flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-border px-4 py-3 md:px-5',
          team === 'A' ? 'bg-team-a/14' : 'bg-team-b/13',
        )}
      >
        <div className="flex flex-wrap items-center gap-3">
          <h2 id={headId} className="m-0 text-[15px]">
            <TeamName team={team} />
          </h2>
          <OutcomeBadge result={own > other ? 'win' : own < other ? 'loss' : 'draw'} />
          {firstSide && <TeamSides first={firstSide} />}
        </div>
        <span className="text-small text-fg-muted tabular-nums">
          Всего K {sum((p) => p.kills)} · D {sum((p) => p.deaths)} · A {sum((p) => p.assists)}
        </span>
      </div>
      {detail ? (
        <StatTable
          columns={DETAIL_COLUMNS as Column<DetailPlayer>[]}
          rows={sortRows(
            detail.filter((p) => p.team === team),
            DETAIL_COLUMNS,
            sort,
          )}
          rowKey={(p) => p.steamId}
          sort={sort}
          onSort={onSort}
          labelledBy={headId}
          minWidth={1060}
        />
      ) : (
        <StatTable
          columns={PLAYER_MATCH_COLUMNS}
          rows={sortRows(rows, PLAYER_MATCH_COLUMNS, sort)}
          rowKey={(p) => p.steamId}
          sort={sort}
          onSort={onSort}
          labelledBy={headId}
          minWidth={PLAYER_MATCH_MIN_WIDTH}
        />
      )}
    </Card>
  )
}
