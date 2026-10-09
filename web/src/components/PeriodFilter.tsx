import { useEffect, useState } from 'react'
import type { SetURLSearchParams } from 'react-router'
import { api, sessionTitle, type SessionSummary } from '../api'
import { updateParams } from '../filters'

// Режим периода хранится в URL: period=sessions или наличие session — выбор сессий, иначе даты.
// Параметр period сервер игнорирует, он нужен, чтобы режим не сбрасывался, пока сессии не выбраны.
function periodMode(params: URLSearchParams): 'dates' | 'sessions' {
  return params.get('period') === 'sessions' || params.has('session') ? 'sessions' : 'dates'
}

// Фильтр периода: либо диапазон дат сессий, либо выбранные сессии.
export function PeriodFilter({ params, setParams }: { params: URLSearchParams; setParams: SetURLSearchParams }) {
  const [sessions, setSessions] = useState<SessionSummary[]>([])
  useEffect(() => {
    api.listSessions().then(setSessions, () => setSessions([]))
  }, [])

  const mode = periodMode(params)
  const selected = new Set(params.getAll('session'))
  const update = (change: (p: URLSearchParams) => void) => updateParams(params, setParams, change)

  function setMode(m: 'dates' | 'sessions') {
    update((p) => {
      p.delete('from')
      p.delete('to')
      p.delete('session')
      p.delete('period')
      if (m === 'sessions') p.set('period', 'sessions')
    })
  }

  function setDate(key: 'from' | 'to', value: string) {
    update((p) => (value ? p.set(key, value) : p.delete(key)))
  }

  function toggleSession(id: string) {
    update((p) => {
      const ids = p.getAll('session').filter((x) => x !== id)
      if (!selected.has(id)) ids.push(id)
      p.delete('session')
      for (const x of ids) p.append('session', x)
    })
  }

  return (
    <fieldset>
      <legend>Период</legend>
      <label>
        <input type="radio" checked={mode === 'dates'} onChange={() => setMode('dates')} /> даты
      </label>{' '}
      <label>
        <input type="radio" checked={mode === 'sessions'} onChange={() => setMode('sessions')} /> сессии
      </label>
      {mode === 'dates' ? (
        <p>
          <label>
            с <input type="date" value={params.get('from') ?? ''} onChange={(e) => setDate('from', e.target.value)} />
          </label>{' '}
          <label>
            по <input type="date" value={params.get('to') ?? ''} onChange={(e) => setDate('to', e.target.value)} />
          </label>
        </p>
      ) : (
        <div className="checklist">
          {sessions.length === 0 && <span className="muted">Сессий нет.</span>}
          {sessions.map((s) => (
            <label key={s.id}>
              <input type="checkbox" checked={selected.has(String(s.id))} onChange={() => toggleSession(String(s.id))} />{' '}
              {s.date}
              {s.title ? ` — ${sessionTitle(s)}` : ''} <span className="muted">({s.matchCount})</span>
            </label>
          ))}
        </div>
      )}
    </fieldset>
  )
}
