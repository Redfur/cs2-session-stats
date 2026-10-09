import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { api, isProcessing, matchStatusLabel, sessionTitle, type Session, type SessionDetails, type UploadResult } from '../api'
import { PlayersTable } from '../components/PlayersTable'

const POLL_MS = 3000
const uploadLabel = { accepted: 'принят', duplicate: 'уже загружен', error: 'ошибка' } as const

export function SessionPage() {
  const { id = '' } = useParams()
  const [data, setData] = useState<SessionDetails | null>(null)
  const [error, setError] = useState('')
  // идёт запрос перестановки или удаления — кнопки действий неактивны
  const [busy, setBusy] = useState(false)
  // ошибка действия над матчами живёт отдельно: load() после действия сбрасывает error
  const [actionError, setActionError] = useState('')
  const navigate = useNavigate()

  const load = useCallback(() => {
    api.getSession(id).then(
      (d) => {
        setData(d)
        setError('')
      },
      (e: Error) => setError(e.message),
    )
  }, [id])

  useEffect(load, [load])

  // пока есть матчи в обработке — опрашиваем сервер
  const inProgress = data?.matches.some(isProcessing) ?? false
  useEffect(() => {
    if (!inProgress) return
    const t = setInterval(load, POLL_MS)
    return () => clearInterval(t)
  }, [inProgress, load])

  if (error && !data) return <p className="error">{error}</p>
  if (!data) return <p className="muted">Загрузка…</p>

  const { session, matches, players } = data

  async function reparseAll() {
    if (!confirm('Пересчитать все матчи сессии? Пока идёт пересчёт, показываются прежние данные.')) return
    try {
      await api.reparseSession(id)
      load()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  // run выполняет действие над матчами и перезагружает сессию — и при успехе, и при ошибке:
  // ошибка 400 у перестановки означает, что список на странице устарел
  async function run(action: () => Promise<unknown>) {
    setBusy(true)
    setActionError('')
    try {
      await action()
    } catch (err) {
      setActionError((err as Error).message)
    } finally {
      setBusy(false)
      load()
    }
  }

  function move(index: number, delta: number) {
    const ids = matches.map((m) => m.id)
    ;[ids[index], ids[index + delta]] = [ids[index + delta], ids[index]]
    run(() => api.reorderMatches(id, ids))
  }

  function removeMatch(ordinal: number, matchId: number) {
    if (!confirm(`Удалить матч #${ordinal} вместе с демкой? Это необратимо.`)) return
    run(() => api.deleteMatch(matchId))
  }

  async function removeSession() {
    if (!confirm(`Удалить сессию и ${matches.length} матч(ей) вместе с демками? Это необратимо.`)) return
    setBusy(true)
    try {
      await api.deleteSession(id)
      navigate('/')
    } catch (err) {
      setActionError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <>
      <SessionHeader
        session={session}
        onSaved={(s) => setData((d) => (d ? { ...d, session: s } : d))}
        onDelete={removeSession}
        busy={busy}
      />

      <UploadForm sessionId={id} onUploaded={load} />

      <h3>Матчи</h3>
      {error && <p className="error">{error}</p>}
      {actionError && <p className="error">{actionError}</p>}
      {matches.length > 0 && (
        <p>
          <button onClick={reparseAll} disabled={matches.every(isProcessing)}>
            Пересчитать все матчи
          </button>
        </p>
      )}
      {matches.length === 0 ? (
        <p className="muted">Матчей пока нет.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>#</th>
              <th>Карта</th>
              <th>Счёт</th>
              <th>Статус</th>
              <th>Файл</th>
              <th>Действия</th>
            </tr>
          </thead>
          <tbody>
            {matches.map((m, i) => (
              <tr key={m.id}>
                <td>
                  <Link to={`/matches/${m.id}`}>{m.ordinal}</Link>
                </td>
                <td>{m.map || '—'}</td>
                <td>{m.hasResult ? `${m.scoreA}:${m.scoreB}` : '—'}</td>
                <td className={m.status === 'failed' ? 'error' : ''}>
                  {matchStatusLabel(m)}
                  {m.error ? `: ${m.error}` : ''}
                </td>
                <td className="muted">{m.originalName}</td>
                <td>
                  <button onClick={() => move(i, -1)} disabled={busy || i === 0} title="Выше">
                    ↑
                  </button>{' '}
                  <button onClick={() => move(i, 1)} disabled={busy || i === matches.length - 1} title="Ниже">
                    ↓
                  </button>{' '}
                  <button onClick={() => removeMatch(m.ordinal, m.id)} disabled={busy}>
                    Удалить
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <h3>Итоги сессии</h3>
      <PlayersTable players={players} aggregate />
    </>
  )
}

function SessionHeader({
  session,
  onSaved,
  onDelete,
  busy,
}: {
  session: Session
  onSaved: (s: Session) => void
  onDelete: () => void
  busy: boolean
}) {
  const [editing, setEditing] = useState(false)
  const [date, setDate] = useState(session.date)
  const [title, setTitle] = useState(session.title)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  function startEdit() {
    setDate(session.date)
    setTitle(session.title)
    setError('')
    setEditing(true)
  }

  async function save(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError('')
    try {
      onSaved(await api.updateSession(String(session.id), date, title))
      setEditing(false)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setSaving(false)
    }
  }

  if (!editing) {
    return (
      <>
        <h2>{sessionTitle(session)}</h2>
        <p className="muted">
          {session.date}{' '}
          <button onClick={startEdit} disabled={busy}>
            Изменить
          </button>{' '}
          <button onClick={onDelete} disabled={busy}>
            Удалить сессию
          </button>
        </p>
      </>
    )
  }
  return (
    <form onSubmit={save}>
      <h2>Редактирование сессии</h2>
      <label>
        Дата <input type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
      </label>{' '}
      <label>
        Название <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="необязательно" maxLength={200} />
      </label>{' '}
      <button disabled={saving}>Сохранить</button>{' '}
      <button type="button" onClick={() => setEditing(false)} disabled={saving}>
        Отмена
      </button>
      {error && <p className="error">{error}</p>}
    </form>
  )
}

function UploadForm({ sessionId, onUploaded }: { sessionId: string; onUploaded: () => void }) {
  const [files, setFiles] = useState<File[]>([])
  const [progress, setProgress] = useState<number | null>(null)
  const [results, setResults] = useState<UploadResult[]>([])
  const [error, setError] = useState('')

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (files.length === 0) return
    const form = e.currentTarget // после await React обнуляет currentTarget
    setError('')
    setResults([])
    setProgress(0)
    try {
      const res: UploadResult[] = []
      for (const f of files) res.push(await api.uploadDemo(sessionId, f, (loaded, total) => setProgress(total ? loaded / total : 0)))
      setResults(res)
      setFiles([])
      form.reset()
      onUploaded()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setProgress(null)
    }
  }

  return (
    <form onSubmit={submit}>
      <h3>Загрузить демки</h3>
      <p className="muted">Форматы: .dem, .dem.gz, .dem.bz2, .dem.zst, .zip. Матчи нумеруются в порядке файлов.</p>
      <input
        type="file"
        multiple
        accept=".dem,.gz,.bz2,.zst,.zip"
        onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
        disabled={progress !== null}
      />{' '}
      <button disabled={files.length === 0 || progress !== null}>Загрузить</button>
      {files.length > 1 && (
        <ol>
          {files.map((f, i) => (
            <li key={i}>{f.name}</li>
          ))}
        </ol>
      )}
      {progress !== null && <p>Загрузка… {Math.round(progress * 100)}%</p>}
      {error && <p className="error">{error}</p>}
      {results.length > 0 && (
        <ul>
          {results.map((r, i) => (
            <li key={i} className={r.status === 'error' ? 'error' : ''}>
              {r.fileName}: {uploadLabel[r.status]}
              {r.status === 'duplicate' && r.sessionId !== undefined && (
                <>
                  {' '}
                  (<Link to={`/matches/${r.matchId}`}>матч</Link> в <Link to={`/sessions/${r.sessionId}`}>сессии</Link>)
                </>
              )}
              {r.error ? ` — ${r.error}` : ''}
            </li>
          ))}
        </ul>
      )}
    </form>
  )
}
