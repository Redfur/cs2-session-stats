// Группа в адресе страницы: адреса страниц группы, список групп и последняя открытая группа.

import { createContext, useContext } from 'react'
import { useParams } from 'react-router'
import type { Group } from './api'

// useGroupId — группа из адреса /g/:gid/…; пустая строка вне страниц группы.
export function useGroupId(): string {
  return useParams().gid ?? ''
}

// Адреса страниц группы. Внутренние ссылки строятся только через них, чтобы не выпасть из группы.
export const paths = {
  sessions: (gid: string) => `/g/${gid}`,
  session: (gid: string, id: string) => `/g/${gid}/sessions/${id}`,
  match: (gid: string, id: string) => `/g/${gid}/matches/${id}`,
  players: (gid: string) => `/g/${gid}/players`,
  // tab — '' (обзор), 'fight', 'weapons' или 'utility'
  player: (gid: string, steamId: string, tab = '') => `/g/${gid}/players/${steamId}${tab ? `/${tab}` : ''}`,
  group: (gid: string) => `/g/${gid}/group`,
  // матч и сессия, группа которых может быть другой (дубликат при загрузке):
  // страница сама перейдёт на адрес своей группы
  anyMatch: (id: string) => `/matches/${id}`,
  anySession: (id: string) => `/sessions/${id}`,
}

const LAST_GROUP_KEY = 'cs2stats.lastGroup'

export function readLastGroup(): string {
  try {
    return localStorage.getItem(LAST_GROUP_KEY) ?? ''
  } catch {
    return ''
  }
}

export function saveLastGroup(gid: string) {
  try {
    localStorage.setItem(LAST_GROUP_KEY, gid)
  } catch {
    /* хранилище недоступно — главная откроет первую группу */
  }
}

// pickGroup — последняя открытая группа, если она ещё есть, иначе первая; undefined — групп нет.
// Сохранённое значение, которого нет в списке (удалённая группа, старый числовой id), заменяется первой группой.
export function pickGroup(groups: Group[]): Group | undefined {
  const last = readLastGroup()
  const found = groups.find((g) => g.id === last)
  if (found) return found
  if (groups[0]) saveLastGroup(groups[0].id)
  return groups[0]
}

export interface GroupsState {
  // null — список ещё не загружен
  groups: Group[] | null
  // ошибка загрузки списка
  error: string
  // перезагружает список; вызывается после создания, переименования и удаления
  reload: () => Promise<Group[]>
}

export const GroupsContext = createContext<GroupsState>({
  groups: null,
  error: '',
  reload: () => Promise.resolve([]),
})

export function useGroups(): GroupsState {
  return useContext(GroupsContext)
}
