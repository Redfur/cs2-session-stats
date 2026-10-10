import { Calendar, Info, Pencil, RefreshCw, Trash } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { api, isImportActive, isProcessing, sessionTitle, type Match, type PlayerRow, type SessionDetails, type SessionExt } from '../api'
import { detailColumns, joinDetails } from '../components/detailColumns'
import { useExtLoad } from '../ext'
import { ImportList } from '../components/ImportList'
import { InlineEdit } from '../components/InlineEdit'
import { MatchLinkForm } from '../components/MatchLinkForm'
import { MatchesTable } from '../components/MatchesTable'
import { SessionDuelsCard } from '../components/Duels'
import { PageError, PageLoading } from '../components/PageState'
import { PLAYER_TOTAL_COLUMNS, PLAYER_TOTAL_MIN_WIDTH, RATING_DESC } from '../components/playerColumns'
import { Alert } from '../components/ui/Alert'
import { Badge } from '../components/ui/Badge'
import { Button } from '../components/ui/Button'
import { Card, CardBody, CardFooter, CardHeader } from '../components/ui/Card'
import { ConfirmDialog } from '../components/ui/Dialog'
import { DropZone } from '../components/ui/DropZone'
import { Page } from '../components/ui/Layout'
import { MetaItem, PageHeader } from '../components/ui/PageHeader'
import { Segment } from '../components/ui/Segment'
import { SortHint, StatTable } from '../components/ui/StatTable'
import { UploadList } from '../components/ui/UploadList'
import { formatDateLong, plural } from '../format'
import { sortRows, useSort, type SortState } from '../sort'
import { useUploadQueue } from '../upload'

const POLL_MS = 3000

const matchesText = (n: number) => plural(n, ['матч', 'матча', 'матчей'])
const playersText = (n: number) => plural(n, ['игрок', 'игрока', 'игроков'])

// пересчитываются матчи с прежним результатом в статусе pending/parsing
const reparsing = (m: Match) => m.hasResult && isProcessing(m)

const DETAIL_COLUMNS = detailColumns('session', {
  old: 'Матчи игрока обработаны старой версией — пересчитайте их',
  noDamage: 'В демках нет событий урона — не посчитано',
  noFlash: 'В демках нет событий ослепления — не посчитано',
})

type TotalsView = 'basic' | 'detail'

// «матч #5 обработан» / «матчи #4 и #5 обработаны»
const oldMatches = (list: { ordinal: number }[]) =>
  list.length === 1 ? `матч ${ordinalList(list)} обработан` : `матчи ${ordinalList(list)} обработаны`

const ordinalList = (list: { ordinal: number }[]) =>
  list
    .map((m) => `#${m.ordinal}`)
    .join(', ')
    .replace(/, ([^,]*)$/, ' и $1')

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

  // пока есть матчи в обработке или скачивания по ссылкам — опрашиваем сервер
  const inProgress = (data?.matches.some(isProcessing) || data?.imports.some(isImportActive)) ?? false
  useEffect(() => {
    if (!inProgress) return
    const t = setInterval(load, POLL_MS)
    return () => clearInterval(t)
  }, [inProgress, load])

  const { sort, toggle } = useSort('sort', PLAYER_TOTAL_COLUMNS, RATING_DESC)
  const detailSort = useSort('sort', DETAIL_COLUMNS, RATING_DESC)
  const [params, setParams] = useSearchParams()
  const totalsView: TotalsView = params.get('totals') === 'detail' ? 'detail' : 'basic'
  const setTotalsView = (v: TotalsView) =>
    setParams(
      (prev) => {
        const p = new URLSearchParams(prev)
        if (v === 'detail') p.set('totals', 'detail')
        else p.delete('totals')
        p.delete('sort') // у видов разные колонки
        return p
      },
      { replace: true },
    )
  const extVersion = data?.matches.map((m) => `${m.id}:${m.status}:${m.processedVersion}`).join(',') ?? ''
  const ext = useExtLoad<SessionExt>(() => api.getSessionExt(id), `${id}|${extVersion}`)

  if (error && !data)
    return <PageError title="Не удалось открыть сессию" error={error} onRetry={load} back={{ to: '/', label: 'К списку сессий' }} />
  if (!data) return <PageLoading />

  const { session, matches, players, imports } = data
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

  const ordinalOf = (matchId: number) => {
    const i = matches.findIndex((m) => m.id === matchId)
    return i >= 0 ? i + 1 : undefined
  }
  const uploadList = (
    <UploadList
      items={uploads.items}
      nextNumber={nextNumber}
      sessionId={session.id}
      ordinalOf={ordinalOf}
      onCancel={uploads.cancel}
      onCancelAll={uploads.cancelAll}
      onClear={uploads.clear}
    />
  )

  const importList = (
    <ImportList imports={imports} ordinalOf={ordinalOf} onChanged={load} onError={(title, text) => setActionError({ title, text })} />
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
          <MatchLinkForm sessionId={id} variant="large" onAdded={load} />
          {(uploads.items.length > 0 || imports.length > 0) && (
            <Card as="div">
              {uploadList}
              {importList}
            </Card>
          )}
        </>
      )}

      {players.length > 0 && (
        <Card aria-labelledby="totals-h">
          <CardHeader
            title="Итоги сессии"
            titleId="totals-h"
            note={totalsNote}
            aside={
              <div className="flex flex-wrap items-center gap-3">
                {totalsView === 'detail' && ext.data?.status === 'partial' && (
                  <span title={`Матчи ${ordinalList(ext.data.uncoveredMatches)} обработаны до обновления`}>
                    <Badge tone="neutral" icon={<Info aria-hidden />}>
                      по {plural(ext.data.coveredMatches, ['матчу', 'матчам', 'матчам'])} из {ext.data.eligibleMatches}
                    </Badge>
                  </span>
                )}
                <span className="hidden md:inline">
                  {totalsView === 'detail' ? (
                    <SortHint columns={DETAIL_COLUMNS} sort={detailSort.sort} prefix="" />
                  ) : (
                    <SortHint columns={PLAYER_TOTAL_COLUMNS} sort={sort} prefix="" />
                  )}
                </span>
                <Segment
                  label="Вид итогов"
                  value={totalsView}
                  onChange={setTotalsView}
                  options={[
                    { value: 'basic', label: 'Основное' },
                    { value: 'detail', label: 'Подробно' },
                  ]}
                />
              </div>
            }
          />
          {totalsView === 'detail' ? (
            <SessionDetailTotals
              ext={ext.data}
              error={ext.error}
              onRetry={ext.reload}
              players={players}
              sort={detailSort}
              reparsing={isReparsing}
              onReparse={async (ids) => {
                setActionError(null)
                try {
                  for (const matchId of ids) await api.reparseMatch(matchId)
                } catch (err) {
                  setActionError({ title: 'Не удалось поставить пересчёт.', text: (err as Error).message })
                }
                load()
              }}
            />
          ) : (
            <StatTable
              columns={PLAYER_TOTAL_COLUMNS}
              rows={sortedPlayers}
              rowKey={(p) => p.steamId}
              sort={sort}
              onSort={toggle}
              labelledBy="totals-h"
              minWidth={PLAYER_TOTAL_MIN_WIDTH}
            />
          )}
        </Card>
      )}

      {withResult > 0 && (
        <SessionDuelsCard
          sessionId={id}
          version={matches.map((m) => `${m.id}:${m.status}:${m.processedVersion}`).join(',')}
          reparsing={isReparsing}
          onReparse={async (ids) => {
            setActionError(null)
            try {
              for (const matchId of ids) await api.reparseMatch(matchId)
            } catch (err) {
              setActionError({ title: 'Не удалось поставить пересчёт.', text: (err as Error).message })
            }
            load()
          }}
        />
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
          {importList}
          <CardFooter>
            <div className="flex flex-col gap-4">
              <DropZone variant="strip" onFiles={uploads.add} nextNumber={nextNumber} rejected={uploads.rejected} />
              <MatchLinkForm sessionId={id} variant="strip" onAdded={load} />
            </div>
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

interface SessionDetailTotalsProps {
  ext: SessionExt | null
  error: string
  onRetry: () => void
  players: PlayerRow[]
  sort: { sort: SortState; toggle: (key: string) => void }
  reparsing: boolean
  onReparse: (ids: number[]) => Promise<void>
}

// SessionDetailTotals — итоги сессии «Подробно»: размены, клатчи, гранаты, выживание.
function SessionDetailTotals({ ext, error, onRetry, players, sort, reparsing, onReparse }: SessionDetailTotalsProps) {
  const [busy, setBusy] = useState(false)
  if (error && !ext)
    return (
      <CardBody>
        <Alert tone="error" title="Не удалось загрузить подробные итоги." action={<Button size="sm" onClick={onRetry}>Повторить</Button>}>
          Сервер не ответил. Основные итоги доступны в режиме «Основное».
        </Alert>
      </CardBody>
    )
  if (!ext)
    return (
      <CardBody>
        <p className="m-0 text-[13px] text-fg-muted" role="status">
          Загружаем подробные итоги…
        </p>
      </CardBody>
    )
  const uncovered = ext.uncoveredMatches
  const covered = ext.coveredMatches
  const reparse = async () => {
    setBusy(true)
    await onReparse(uncovered.map((m) => m.id)).finally(() => setBusy(false))
  }
  const rows = sortRows(joinDetails(players, ext.players), DETAIL_COLUMNS, sort.sort)
  return (
    <>
      {uncovered.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3 text-small text-fg-muted md:px-5">
          <span>
            {covered > 0
              ? `Подробные колонки — по ${plural(covered, ['матчу', 'матчам', 'матчам'])} из ${ext.eligibleMatches}: ${oldMatches(uncovered)} до обновления.`
              : `${oldMatches(uncovered)} до появления подробной статистики.`}
          </span>
          <Button size="sm" leading={<RefreshCw aria-hidden />} loading={busy || reparsing} onClick={reparse}>
            {reparsing ? 'Пересчитываем…' : 'Пересчитать их'}
          </Button>
        </div>
      )}
      <StatTable
        columns={DETAIL_COLUMNS}
        rows={rows}
        rowKey={(p) => p.steamId}
        sort={sort.sort}
        onSort={sort.toggle}
        labelledBy="totals-h"
        minWidth={1040}
      />
      <CardFooter className="text-small text-fg-muted">
        Урон — за раунд. Размен — месть за союзника не позже 5 с. Клатчи — выиграно / попыток. Гранаты — урон HE и огнём в
        среднем за карту. Новые колонки шкалой не окрашиваются.
      </CardFooter>
    </>
  )
}
