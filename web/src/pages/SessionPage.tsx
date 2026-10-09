import { Calendar, Pencil, RefreshCw, Trash } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { api, isProcessing, sessionTitle, type Match, type SessionDetails } from '../api'
import { InlineEdit } from '../components/InlineEdit'
import { MatchesTable } from '../components/MatchesTable'
import { PageError, PageLoading } from '../components/PageState'
import { PLAYER_TOTAL_COLUMNS, PLAYER_TOTAL_MIN_WIDTH, RATING_DESC } from '../components/playerColumns'
import { Alert } from '../components/ui/Alert'
import { Button } from '../components/ui/Button'
import { Card, CardBody, CardFooter, CardHeader } from '../components/ui/Card'
import { ConfirmDialog } from '../components/ui/Dialog'
import { DropZone } from '../components/ui/DropZone'
import { Page } from '../components/ui/Layout'
import { MetaItem, PageHeader } from '../components/ui/PageHeader'
import { SortHint, StatTable } from '../components/ui/StatTable'
import { UploadList } from '../components/ui/UploadList'
import { formatDateLong, plural } from '../format'
import { sortRows, useSort } from '../sort'
import { useUploadQueue } from '../upload'

const POLL_MS = 3000

const matchesText = (n: number) => plural(n, ['матч', 'матча', 'матчей'])
const playersText = (n: number) => plural(n, ['игрок', 'игрока', 'игроков'])

// пересчитываются матчи с прежним результатом в статусе pending/parsing
const reparsing = (m: Match) => m.hasResult && isProcessing(m)

type Confirm = { kind: 'session' } | { kind: 'reparse' } | { kind: 'match'; match: Match; number: number }

export function SessionPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [data, setData] = useState<SessionDetails | null>(null)
  const [error, setError] = useState('')
  // ошибка действия над матчами живёт отдельно: load() после действия сбрасывает error
  const [actionError, setActionError] = useState<{ title: string; text: string } | null>(null)
  // id матчей, замеченных в пересчёте с момента, когда пересчитываемых не было (плашка прогресса)
  const [reparseIds, setReparseIds] = useState<number[]>([])
  const [editing, setEditing] = useState<'title' | 'date' | null>(null)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const [movedId, setMovedId] = useState<number | null>(null)
  const [orderStatus, setOrderStatus] = useState('')
  // идёт сохранение порядка: ответы опроса не перетирают локальный порядок
  const saving = useRef(false)
  const pendingOrder = useRef<number[] | null>(null)

  const load = useCallback(() => {
    api.getSession(id).then(
      (d) => {
        if (saving.current) return
        setData(d)
        setError('')
        const now = d.matches.filter(reparsing).map((m) => m.id)
        setReparseIds((prev) => (now.length === 0 ? [] : [...new Set([...prev, ...now])]))
      },
      (e: Error) => setError(e.message),
    )
  }, [id])

  useEffect(load, [load])

  const uploads = useUploadQueue(id, load)

  // пока есть матчи в обработке — опрашиваем сервер
  const inProgress = data?.matches.some(isProcessing) ?? false
  useEffect(() => {
    if (!inProgress) return
    const t = setInterval(load, POLL_MS)
    return () => clearInterval(t)
  }, [inProgress, load])

  const { sort, toggle } = useSort('sort', PLAYER_TOTAL_COLUMNS, RATING_DESC)

  if (error && !data)
    return <PageError title="Не удалось открыть сессию" error={error} onRetry={load} back={{ to: '/', label: 'К списку сессий' }} />
  if (!data) return <PageLoading />

  const { session, matches, players } = data
  const title = sessionTitle(session)
  const withResult = matches.filter((m) => m.hasResult).length
  const nowReparsing = matches.filter(reparsing).length
  const isReparsing = nowReparsing > 0
  const nextNumber = matches.length + 1

  // Сохраняет порядок полным списком матчей. Запросы идут последовательно, на сервер уходит
  // последний порядок. Если сервер отклонил список (например, он устарел), показываем ошибку
  // и перезагружаем актуальный порядок.
  async function saveOrder(ids: number[]) {
    pendingOrder.current = ids
    if (saving.current) return
    saving.current = true
    let failed = false
    while (pendingOrder.current) {
      const next = pendingOrder.current
      pendingOrder.current = null
      try {
        await api.reorderMatches(id, next)
      } catch (err) {
        pendingOrder.current = null
        failed = true
        setActionError({ title: 'Не удалось сохранить порядок.', text: (err as Error).message })
        setOrderStatus('')
      }
    }
    saving.current = false
    if (!failed) setOrderStatus('Порядок сохранён')
    load()
  }

  function move(from: number, to: number) {
    const list = [...matches]
    const [m] = list.splice(from, 1)
    list.splice(to, 0, m)
    setData({ ...data!, matches: list })
    setMovedId(m.id)
    setActionError(null)
    setOrderStatus(`Матч ${m.map || m.originalName} перемещён на позицию ${to + 1} из ${list.length}`)
    saveOrder(list.map((x) => x.id))
  }

  async function saveSession(date: string, newTitle: string) {
    const s = await api.updateSession(id, date, newTitle)
    setData((d) => (d ? { ...d, session: s } : d))
  }

  const reparseBanner = isReparsing && (
    <Alert
      tone="info"
      title={`Пересчитываем ${matchesText(reparseIds.length)} — готово ${reparseIds.length - nowReparsing} из ${reparseIds.length}.`}
    >
      Пока показываем прежние счёт и статистику, страница обновится сама.
    </Alert>
  )

  const sortedPlayers = sortRows(players, PLAYER_TOTAL_COLUMNS, sort)
  const totalsNote = isReparsing
    ? 'прежние данные — обновятся после пересчёта'
    : withResult < matches.length
      ? `${playersText(players.length)} · по ${withResult} ${withResult % 10 === 1 && withResult % 100 !== 11 ? 'готовому матчу' : 'готовым матчам'} из ${matches.length}`
      : `${playersText(players.length)} · ${matchesText(matches.length)}`

  const processingNumbers = matches.flatMap((m, i) => (!m.hasResult && isProcessing(m) ? [`#${i + 1}`] : []))

  const uploadList = (
    <UploadList
      items={uploads.items}
      nextNumber={nextNumber}
      sessionId={session.id}
      ordinalOf={(matchId) => {
        const i = matches.findIndex((m) => m.id === matchId)
        return i >= 0 ? i + 1 : undefined
      }}
      onCancel={uploads.cancel}
      onCancelAll={uploads.cancelAll}
      onClear={uploads.clear}
    />
  )

  return (
    <Page>
      <PageHeader
        back={{ to: '/', label: 'Все сессии' }}
        titleSlot={
          editing === 'title' ? (
            <InlineEdit
              kind="title"
              label="Название сессии"
              value={session.title}
              placeholder="без названия — покажем дату"
              onSave={(v) => saveSession(session.date, v)}
              onDone={() => setEditing(null)}
            />
          ) : (
            <>
              <h1 className="m-0 min-w-0 text-[24px] leading-[30px] font-bold tracking-[-0.01em] break-words md:text-page">{title}</h1>
              {editing === null && (
                <Button variant="ghost" size="sm" icon aria-label="Изменить название" title="Изменить название" onClick={() => setEditing('title')}>
                  <Pencil aria-hidden />
                </Button>
              )}
            </>
          )
        }
        meta={
          <>
            {editing === 'date' ? (
              <InlineEdit
                kind="date"
                label="Дата сессии"
                value={session.date}
                icon={<Calendar className="size-4" aria-hidden />}
                onSave={(v) => saveSession(v, session.title)}
                onDone={() => setEditing(null)}
              />
            ) : (
              <MetaItem>
                <Calendar aria-hidden />
                {formatDateLong(session.date)}
                {editing === null && (
                  <Button variant="ghost" size="xs" icon aria-label="Изменить дату" title="Изменить дату" onClick={() => setEditing('date')}>
                    <Pencil aria-hidden />
                  </Button>
                )}
              </MetaItem>
            )}
            <MetaItem className="tabular-nums">
              {matches.length > 0 ? matchesText(matches.length) : 'нет матчей'}
              {players.length > 0 && ` · ${playersText(players.length)}`}
            </MetaItem>
            {!session.title && <MetaItem className="text-fg-faint">без названия</MetaItem>}
          </>
        }
        actions={
          <Button variant="danger-quiet" icon aria-label="Удалить сессию" title="Удалить сессию" onClick={() => setConfirm({ kind: 'session' })}>
            <Trash aria-hidden />
          </Button>
        }
      />

      {error && (
        <Alert
          tone="error"
          title="Не удалось обновить данные сессии."
          action={
            <Button size="sm" onClick={load}>
              Повторить
            </Button>
          }
        >
          {error}. Данные на странице могут быть устаревшими.
        </Alert>
      )}
      {reparseBanner}

      {matches.length === 0 && (
        <>
          <DropZone variant="large" onFiles={uploads.add} nextNumber={nextNumber} rejected={uploads.rejected} />
          {uploads.items.length > 0 && <Card as="div">{uploadList}</Card>}
        </>
      )}

      {players.length > 0 && (
        <Card aria-labelledby="totals-h">
          <CardHeader
            title="Итоги сессии"
            titleId="totals-h"
            note={totalsNote}
            aside={
              <span className="hidden md:inline">
                <SortHint columns={PLAYER_TOTAL_COLUMNS} sort={sort} prefix="" />
                <span className="text-small text-fg-muted"> · нажмите на колонку, чтобы сортировать</span>
              </span>
            }
          />
          <StatTable
            columns={PLAYER_TOTAL_COLUMNS}
            rows={sortedPlayers}
            rowKey={(p) => p.steamId}
            sort={sort}
            onSort={toggle}
            labelledBy="totals-h"
            minWidth={PLAYER_TOTAL_MIN_WIDTH}
          />
        </Card>
      )}

      {actionError && (
        <Alert tone="error" title={actionError.title}>
          {actionError.text}
        </Alert>
      )}

      {matches.length > 0 && (
        <Card aria-labelledby="matches-h">
          <CardHeader
            title="Матчи"
            titleId="matches-h"
            note={
              <>
                {matches.length}
                <span className="ml-3" role="status" aria-live="polite">
                  {orderStatus}
                </span>
              </>
            }
            aside={
              isReparsing ? (
                <Button size="sm" loading aria-disabled leading={<RefreshCw aria-hidden />}>
                  Пересчитываем…
                </Button>
              ) : (
                <Button size="sm" leading={<RefreshCw aria-hidden />} onClick={() => setConfirm({ kind: 'reparse' })}>
                  Пересчитать все
                </Button>
              )
            }
          />
          <MatchesTable
            matches={matches}
            labelledBy="matches-h"
            movedId={movedId}
            onMove={move}
            onDelete={(match, number) => setConfirm({ kind: 'match', match, number })}
          />
          {uploadList}
          <CardFooter>
            <DropZone variant="strip" onFiles={uploads.add} nextNumber={nextNumber} rejected={uploads.rejected} />
          </CardFooter>
        </Card>
      )}

      {players.length === 0 && (
        <Card>
          <CardHeader title="Итоги сессии" />
          <CardBody>
            <p className="m-0 text-[13px] text-fg-muted">
              Нет данных — итоги появятся, когда хотя бы один матч будет в статусе «Готово».
              {processingNumbers.length > 0 &&
                ` ${processingNumbers.length === 1 ? 'Матч' : 'Матчи'} ${processingNumbers.join(', ')} сейчас ${processingNumbers.length === 1 ? 'обрабатывается' : 'обрабатываются'}.`}
            </p>
          </CardBody>
        </Card>
      )}

      <ConfirmDialog
        open={confirm?.kind === 'session'}
        onClose={() => setConfirm(null)}
        tone="danger"
        title="Удалить сессию?"
        description={
          <>
            Сессия <b>{title}</b>
            {matches.length > 0 ? ` и ${matchesText(matches.length)} в ней будут удалены` : ' будет удалена'} вместе с демками и
            статистикой. Отменить это нельзя.
          </>
        }
        confirmLabel="Удалить сессию"
        errorTitle="Не удалось удалить сессию."
        onConfirm={async () => {
          await api.deleteSession(id)
          navigate('/')
        }}
      />
      <ConfirmDialog
        open={confirm?.kind === 'reparse'}
        onClose={() => setConfirm(null)}
        tone="normal"
        title="Пересчитать все матчи?"
        description={
          <>
            {matchesText(matches.length)} снова {matches.length === 1 ? 'попадёт' : 'попадут'} в очередь на разбор. Пока идёт пересчёт, на
            странице остаются прежние счёт и статистика.
          </>
        }
        confirmLabel="Пересчитать"
        errorTitle="Не удалось поставить пересчёт."
        onConfirm={async () => {
          await api.reparseSession(id)
          load()
        }}
      />
      <ConfirmDialog
        open={confirm?.kind === 'match'}
        onClose={() => setConfirm(null)}
        tone="danger"
        title="Удалить матч?"
        description={
          confirm?.kind === 'match' && (
            <>
              Матч{' '}
              <b>
                #{confirm.number}
                {confirm.match.map && ` — ${confirm.match.map}`}
              </b>{' '}
              будет удалён вместе с демкой и статистикой. Отменить это нельзя.
            </>
          )
        }
        confirmLabel="Удалить матч"
        errorTitle="Не удалось удалить матч."
        onConfirm={async () => {
          if (confirm?.kind !== 'match') return
          await api.deleteMatch(confirm.match.id)
          load()
        }}
      />
    </Page>
  )
}
