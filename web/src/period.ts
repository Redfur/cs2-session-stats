// Фильтр периода страниц игроков: режим, даты и выбранные сессии хранятся в адресе страницы.

import { useEffect, useState } from 'react'
import type { SetURLSearchParams } from 'react-router'
import { api, type SessionSummary } from './api'
import { updateParams } from './filters'
import { formatDateShort, plural } from './format'

export type PeriodMode = 'dates' | 'sessions'

// Режим хранится в URL: period=sessions или наличие session — выбор сессий, иначе даты.
// Параметр period сервер игнорирует, он нужен, чтобы режим не сбрасывался, пока сессии не выбраны.
export function periodMode(params: URLSearchParams): PeriodMode {
  return params.get('period') === 'sessions' || params.has('session') ? 'sessions' : 'dates'
}

export function usePeriod(params: URLSearchParams, setParams: SetURLSearchParams) {
  const update = (change: (p: URLSearchParams) => void) => updateParams(params, setParams, change)
  const selected = new Set(params.getAll('session'))
  return {
    mode: periodMode(params),
    from: params.get('from') ?? '',
    to: params.get('to') ?? '',
    selected,
    setMode(m: PeriodMode) {
      update((p) => {
        p.delete('from')
        p.delete('to')
        p.delete('session')
        p.delete('period')
        if (m === 'sessions') p.set('period', 'sessions')
      })
    },
    setDate(key: 'from' | 'to', value: string) {
      update((p) => (value ? p.set(key, value) : p.delete(key)))
    },
    toggleSession(id: string) {
      update((p) => {
        const ids = p.getAll('session').filter((x) => x !== id)
        if (!selected.has(id)) ids.push(id)
        p.delete('session')
        for (const x of ids) p.append('session', x)
      })
    },
  }
}

export type Period = ReturnType<typeof usePeriod>

// periodText — выбранный период словами: «за всё время», «с 01.10.2026», «2 сессии».
export function periodText(p: Period): string {
  if (p.mode === 'sessions') {
    return p.selected.size ? plural(p.selected.size, ['сессия', 'сессии', 'сессий']) : 'сессии не выбраны — за всё время'
  }
  if (p.from && p.to) return `${formatDateShort(p.from)} — ${formatDateShort(p.to)}`
  if (p.from) return `с ${formatDateShort(p.from)}`
  if (p.to) return `по ${formatDateShort(p.to)}`
  return 'за всё время'
}

// useSessionList загружает список сессий для выбора в фильтре.
export function useSessionList(): SessionSummary[] {
  const [sessions, setSessions] = useState<SessionSummary[]>([])
  useEffect(() => {
    api.listSessions().then(setSessions, () => setSessions([]))
  }, [])
  return sessions
}
