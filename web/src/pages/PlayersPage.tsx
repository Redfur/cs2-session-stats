import { useEffect, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api, type PlayerRow } from '../api'
import { PeriodFilter } from '../components/PeriodFilter'
import { updateParams } from '../filters'
import { PlayersTable } from '../components/PlayersTable'

const DEFAULT_MIN_MATCHES = '3'

export function PlayersPage() {
  const [params, setParams] = useSearchParams()
  const [players, setPlayers] = useState<PlayerRow[] | null>(null)
  const [options, setOptions] = useState<PlayerRow[]>([])
  const [error, setError] = useState('')

  const minMatches = params.get('minMatches') ?? DEFAULT_MIN_MATCHES
  const selected = new Set(params.getAll('player'))
  const update = (change: (p: URLSearchParams) => void) => updateParams(params, setParams, change)

  // порог по умолчанию задаёт интерфейс, поэтому в запрос он передаётся явно
  const query = useMemo(() => {
    const q = new URLSearchParams(params)
    q.set('minMatches', minMatches)
    return q.toString()
  }, [params, minMatches])

  useEffect(() => {
    let stale = false
    api.listPlayers(query).then(
      (r) => {
        if (stale) return
        setPlayers(r.players)
        setError('')
      },
      (e: Error) => !stale && setError(e.message),
    )
    return () => {
      stale = true
    }
  }, [query])

  // варианты мультивыбора — все игроки без фильтров
  useEffect(() => {
    api.listPlayers('').then(
      (r) => setOptions([...r.players].sort((a, b) => a.name.localeCompare(b.name))),
      () => setOptions([]),
    )
  }, [])

  function togglePlayer(id: string) {
    update((p) => {
      const ids = p.getAll('player').filter((x) => x !== id)
      if (!selected.has(id)) ids.push(id)
      p.delete('player')
      for (const x of ids) p.append('player', x)
    })
  }

  function setMinMatches(value: string) {
    const q = new URLSearchParams(params)
    if (value === DEFAULT_MIN_MATCHES) q.delete('minMatches')
    else q.set('minMatches', value)
    setParams(q, { replace: true }) // ввод числа не должен плодить записи истории
  }

  return (
    <>
      <h2>Игроки</h2>
      <PeriodFilter params={params} setParams={setParams} />

      <fieldset>
        <legend>Игроки</legend>
        <div className="checklist">
          {options.length === 0 && <span className="muted">Игроков нет.</span>}
          {options.map((p) => (
            <label key={p.steamId}>
              <input type="checkbox" checked={selected.has(p.steamId)} onChange={() => togglePlayer(p.steamId)} /> {p.name}{' '}
              <span className="muted">({p.matches ?? 0})</span>
            </label>
          ))}
        </div>
        <label title="Учитывать только матчи, в которых сыграли все выбранные игроки">
          <input
            type="checkbox"
            checked={params.get('together') === '1'}
            disabled={selected.size < 2}
            onChange={(e) => update((p) => (e.target.checked ? p.set('together', '1') : p.delete('together')))}
          />{' '}
          только совместные матчи
        </label>
      </fieldset>

      <p>
        <label>
          Минимум матчей{' '}
          <input type="number" min={1} value={minMatches} onChange={(e) => setMinMatches(e.target.value)} style={{ width: 60 }} />
        </label>{' '}
        <Link to="/players">сбросить фильтры</Link>
      </p>

      {error && <p className="error">{error}</p>}
      {players === null && !error && <p className="muted">Загрузка…</p>}
      {players && <PlayersTable players={players} aggregate />}
    </>
  )
}
