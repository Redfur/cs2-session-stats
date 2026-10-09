import { ChevronLeft, ChevronRight, LoaderCircle, RefreshCw, Trash } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { api, isProcessing, sessionTitle, type MatchDetails, type PlayerRow, type SessionDetails } from '../api'
import { MatchDuelsCard } from '../components/Duels'
import { PageError, PageLoading } from '../components/PageState'
import { PLAYER_MATCH_COLUMNS, PLAYER_MATCH_MIN_WIDTH, RATING_DESC } from '../components/playerColumns'
import { Alert } from '../components/ui/Alert'
import { OutcomeBadge, StatusBadge } from '../components/ui/Badge'
import { Button, ButtonLink } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { cx } from '../components/ui/cx'
import { ConfirmDialog } from '../components/ui/Dialog'
import { Page } from '../components/ui/Layout'
import { MetaItem, PageHeader } from '../components/ui/PageHeader'
import { TeamName } from '../components/ui/Score'
import { ScoreBoard } from '../components/ui/ScoreBoard'
import { StatTable } from '../components/ui/StatTable'
import { plural } from '../format'
import { sortRows, useSort, type SortState } from '../sort'

const POLL_MS = 3000

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

      {match.hasResult && (
        <>
          <ScoreBoard
            scoreA={match.scoreA}
            scoreB={match.scoreB}
            footer={`${plural(match.rounds, ['раунд', 'раунда', 'раундов'])} · Команда A начала матч за CT`}
          />
          <TeamCard team="A" own={match.scoreA} other={match.scoreB} players={players} sort={sort} onSort={toggle} />
          <TeamCard team="B" own={match.scoreB} other={match.scoreA} players={players} sort={sort} onSort={toggle} />
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
  sort: SortState
  onSort: (key: string) => void
}

// TeamCard — шапка команды с исходом и суммами K·D·A и таблица её игроков.
function TeamCard({ team, own, other, players, sort, onSort }: TeamCardProps) {
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
        </div>
        <span className="text-small text-fg-muted tabular-nums">
          Всего K {sum((p) => p.kills)} · D {sum((p) => p.deaths)} · A {sum((p) => p.assists)}
        </span>
      </div>
      <StatTable
        columns={PLAYER_MATCH_COLUMNS}
        rows={sortRows(rows, PLAYER_MATCH_COLUMNS, sort)}
        rowKey={(p) => p.steamId}
        sort={sort}
        onSort={onSort}
        labelledBy={headId}
        minWidth={PLAYER_MATCH_MIN_WIDTH}
      />
    </Card>
  )
}
