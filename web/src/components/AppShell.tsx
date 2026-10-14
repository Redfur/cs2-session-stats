import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useLocation, useMatch } from 'react-router'
import { api, type Group } from '../api'
import { GroupsContext, paths, pickGroup, saveLastGroup } from '../groups'
import { NewGroupDialog } from './NewGroupDialog'
import { GroupSwitcher } from './ui/GroupSwitcher'
import { Layout, type NavItem } from './ui/Layout'

// AppShell загружает список групп и рисует шапку текущей группы: переключатель и навигацию.
// Вне адресов группы (главная, «не найдено») шапка показывает последнюю открытую группу.
export function AppShell({ children }: { children: ReactNode }) {
  const urlGid = useMatch('/g/:gid/*')?.params.gid ?? ''
  const { pathname } = useLocation()
  const [groups, setGroups] = useState<Group[] | null>(null)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)

  const reload = useCallback(
    () =>
      api.listGroups().then(
        (list) => {
          setGroups(list)
          setError('')
          return list
        },
        (e: Error) => {
          setError(e.message)
          throw e
        },
      ),
    [],
  )
  useEffect(() => {
    reload().catch(() => {})
  }, [reload])

  const current = groups?.find((g) => String(g.id) === urlGid)
  useEffect(() => {
    if (current) saveLastGroup(String(current.id))
  }, [current])

  // группа шапки: из адреса, а пока список грузится — тоже из адреса; без неё — последняя открытая
  const shown = current ?? (urlGid ? undefined : groups ? pickGroup(groups) : undefined)
  const gid = shown ? String(shown.id) : groups === null ? urlGid : ''

  const nav: NavItem[] | undefined = gid
    ? [
        {
          to: paths.sessions(gid),
          label: 'Сессии',
          active:
            urlGid === gid &&
            (pathname === paths.sessions(gid) ||
              pathname.startsWith(`${paths.sessions(gid)}/sessions/`) ||
              pathname.startsWith(`${paths.sessions(gid)}/matches/`)),
        },
        { to: paths.players(gid), label: 'Игроки', active: urlGid === gid && pathname.startsWith(paths.players(gid)) },
        { to: paths.group(gid), label: 'Группа', active: urlGid === gid && pathname.startsWith(paths.group(gid)) },
      ]
    : undefined

  const ctx = useMemo(() => ({ groups, error, reload }), [groups, error, reload])
  return (
    <GroupsContext value={ctx}>
      <Layout
        logoTo={gid ? paths.sessions(gid) : undefined}
        nav={nav}
        switcher={
          shown &&
          groups && (
            <GroupSwitcher
              groups={groups}
              current={shown}
              hrefFor={paths.sessions}
              onCreate={() => setCreating(true)}
              onOpen={() => reload().catch(() => {})}
            />
          )
        }
      >
        {children}
      </Layout>
      <NewGroupDialog open={creating} onClose={() => setCreating(false)} />
    </GroupsContext>
  )
}
