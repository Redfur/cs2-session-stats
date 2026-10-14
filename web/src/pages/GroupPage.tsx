import { Pencil, Trash } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { api, ApiError, type GroupSummary } from '../api'
import { InlineEdit } from '../components/InlineEdit'
import { MAX_GROUP_NAME } from '../components/NewGroupDialog'
import { PageError, PageLoading } from '../components/PageState'
import { Button } from '../components/ui/Button'
import { Card, CardHeader } from '../components/ui/Card'
import { ConfirmDialog } from '../components/ui/Dialog'
import { Page } from '../components/ui/Layout'
import { PageHeader } from '../components/ui/PageHeader'
import { plural } from '../format'
import { paths, pickGroup, useGroupId, useGroups } from '../groups'
import { NotFoundPage } from './NotFoundPage'

const sessionsText = (n: number) => plural(n, ['сессия', 'сессии', 'сессий'])

// groupMeta — сводка под названием: «4 сессии · 21 матч · 19 игроков» или «нет сессий».
function groupMeta(g: GroupSummary): string {
  const parts = [g.sessionCount ? sessionsText(g.sessionCount) : 'нет сессий']
  if (g.matchCount) parts.push(plural(g.matchCount, ['матч', 'матча', 'матчей']))
  if (g.playerCount) parts.push(plural(g.playerCount, ['игрок', 'игрока', 'игроков']))
  return parts.join(' · ')
}

// GroupPage — страница группы: название с правкой на месте, сводка и удаление пустой группы.
export function GroupPage() {
  const gid = useGroupId()
  const { reload: reloadGroups } = useGroups()
  const navigate = useNavigate()
  const [data, setData] = useState<GroupSummary | null>(null)
  const [error, setError] = useState('')
  const [notFound, setNotFound] = useState(false)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  // удаление отклонено: пока диалог был открыт, в группу добавили сессию
  const [conflict, setConflict] = useState(false)

  const [prevGid, setPrevGid] = useState(gid)
  if (gid !== prevGid) {
    setPrevGid(gid)
    setData(null)
    setError('')
    setNotFound(false)
    setEditing(false)
  }

  const load = useCallback(() => {
    api.getGroup(gid).then(
      (g) => {
        setData(g)
        setError('')
      },
      (e: Error) => {
        setNotFound(e instanceof ApiError && e.status === 404)
        setError(e.message)
      },
    )
  }, [gid])
  useEffect(load, [load])

  if (notFound) return <NotFoundPage />
  if (error && !data)
    return <PageError title="Не удалось загрузить группу" error={error} onRetry={load} back={{ to: paths.sessions(gid), label: 'К сессиям' }} />
  if (!data) return <PageLoading />

  const empty = data.sessionCount === 0

  async function rename(name: string) {
    const g = await api.renameGroup(gid, name)
    setData((d) => (d ? { ...d, name: g.name } : d))
    await reloadGroups().catch(() => {})
  }

  async function remove() {
    try {
      await api.deleteGroup(gid)
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(true)
        load()
        throw new Error('Пока окно было открыто, в группу добавили сессию.')
      }
      throw e
    }
    const list = await reloadGroups().catch(() => [])
    const next = pickGroup(list)
    navigate(next ? paths.sessions(next.id) : '/')
  }

  return (
    <Page>
      <PageHeader
        titleSlot={
          editing ? (
            <InlineEdit
              kind="title"
              label="Название группы"
              value={data.name}
              maxLength={MAX_GROUP_NAME}
              onSave={rename}
              onDone={() => setEditing(false)}
            />
          ) : (
            <>
              <h1
                className="m-0 min-w-0 truncate text-[24px] leading-[30px] font-bold tracking-[-0.01em] md:text-page"
                title={data.name}
              >
                {data.name}
              </h1>
              <Button variant="ghost" size="sm" icon aria-label="Изменить название" title="Изменить название" onClick={() => setEditing(true)}>
                <Pencil aria-hidden />
              </Button>
            </>
          )
        }
        meta={<span className="tabular-nums">{groupMeta(data)}</span>}
      />

      <Card danger aria-labelledby="dz-h">
        <CardHeader title={<span className="text-metric-bad">Опасная зона</span>} titleId="dz-h" />
        <div className="grid grid-cols-1 items-start gap-x-10 gap-y-3 p-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)] md:p-5">
          <div className="flex min-w-0 flex-col gap-1">
            <b className="font-bold">Удалить группу</b>
            <span className="text-small text-fg-muted">
              {empty
                ? 'В группе нет сессий — её можно удалить. Отменить нельзя.'
                : `Удалить можно только группу без сессий. В этой группе ${sessionsText(data.sessionCount)} — сначала удалите их.`}
            </span>
          </div>
          <div className="flex min-w-0 flex-col items-end">
            <Button
              variant="danger-quiet"
              icon
              aria-label="Удалить группу"
              title={empty ? 'Удалить группу' : 'В группе есть сессии — сначала удалите их'}
              disabled={!empty}
              onClick={() => {
                setConflict(false)
                setDeleting(true)
              }}
            >
              <Trash aria-hidden />
            </Button>
          </div>
        </div>
      </Card>

      <ConfirmDialog
        open={deleting}
        onClose={() => setDeleting(false)}
        tone="danger"
        title={`Удалить группу «${data.name}»?`}
        description="Группа пустая: в ней нет сессий и матчей. После удаления откроется другая группа."
        confirmLabel="Удалить группу"
        errorTitle={conflict ? 'В группе есть сессии — сначала удалите их.' : 'Не удалось удалить группу.'}
        onConfirm={remove}
      />
    </Page>
  )
}
