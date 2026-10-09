import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { api, isProcessing, matchStatusLabel, type MatchDetails } from '../api'
import { PlayersTable } from '../components/PlayersTable'

const POLL_MS = 3000

export function MatchPage() {
  const { id = '' } = useParams()
  const [data, setData] = useState<MatchDetails | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(() => {
    api.getMatch(id).then(
      (d) => {
        setData(d)
        setError('')
      },
      (e: Error) => setError(e.message),
    )
  }, [id])

  useEffect(load, [load])

  // пока матч в очереди или в обработке — опрашиваем сервер
  const processing = data ? isProcessing(data.match) : false
  useEffect(() => {
    if (!processing) return
    const t = setInterval(load, POLL_MS)
    return () => clearInterval(t)
  }, [processing, load])

  async function reparse() {
    try {
      const match = await api.reparseMatch(id)
      setData((d) => (d ? { ...d, match } : d))
    } catch (err) {
      setError((err as Error).message)
    }
  }

  if (error && !data) return <p className="error">{error}</p>
  if (!data) return <p className="muted">Загрузка…</p>

  const { match, players } = data
  return (
    <>
      <p>
        <Link to={`/sessions/${match.sessionId}`}>← к сессии</Link>
      </p>
      <h2>
        Матч #{match.ordinal} {match.map && `— ${match.map}`}
      </h2>
      <p className="muted">{match.originalName}</p>
      <p>
        Статус: {matchStatusLabel(match)}{' '}
        <button onClick={reparse} disabled={processing}>
          Пересчитать
        </button>
      </p>
      {error && <p className="error">{error}</p>}

      {match.status === 'failed' && (
        <p className="error">
          {match.hasResult ? 'Пересчёт не удался, показаны прежние данные' : 'Ошибка обработки'}: {match.error}
        </p>
      )}
      {processing && (
        <p className="muted">{match.hasResult ? 'Идёт пересчёт, показаны прежние данные.' : 'Демка ещё обрабатывается.'}</p>
      )}
      {match.hasResult && (
        <>
          <h3>
            Команда A {match.scoreA} : {match.scoreB} Команда B
          </h3>
          <p className="muted">Раундов: {match.rounds}. Команда A начала матч за CT.</p>
          <h4>Команда A</h4>
          <PlayersTable players={players.filter((p) => p.team === 'A')} />
          <h4>Команда B</h4>
          <PlayersTable players={players.filter((p) => p.team === 'B')} />
        </>
      )}
    </>
  )
}
