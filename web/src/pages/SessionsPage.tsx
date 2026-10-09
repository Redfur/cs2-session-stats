import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { api, sessionTitle, todayISO, type SessionSummary } from '../api'

export function SessionsPage() {
  const [sessions, setSessions] = useState<SessionSummary[] | null>(null)
  const [error, setError] = useState('')
  const [date, setDate] = useState(todayISO())
  const [title, setTitle] = useState('')
  const [creating, setCreating] = useState(false)
  const navigate = useNavigate()

  useEffect(() => {
    api.listSessions().then(setSessions, (e: Error) => setError(e.message))
  }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    setCreating(true)
    setError('')
    try {
      const s = await api.createSession(date, title)
      navigate(`/sessions/${s.id}`)
    } catch (err) {
      setError((err as Error).message)
      setCreating(false)
    }
  }

  return (
    <>
      <h2>Новая сессия</h2>
      <form onSubmit={create}>
        <label>
          Дата <input type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
        </label>{' '}
        <label>
          Название <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="необязательно" maxLength={200} />
        </label>{' '}
        <button disabled={creating}>Создать</button>
      </form>
      {error && <p className="error">{error}</p>}

      <h2>Сессии</h2>
      {sessions === null && !error && <p className="muted">Загрузка…</p>}
      {sessions?.length === 0 && <p className="muted">Сессий пока нет — создайте первую и загрузите в неё демки вечера.</p>}
      {sessions && sessions.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Сессия</th>
              <th>Дата</th>
              <th>Матчей</th>
            </tr>
          </thead>
          <tbody>
            {sessions.map((s) => (
              <tr key={s.id}>
                <td>
                  <Link to={`/sessions/${s.id}`}>{sessionTitle(s)}</Link>
                </td>
                <td>{s.date}</td>
                <td>{s.matchCount}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}
