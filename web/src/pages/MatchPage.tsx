import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { api, type MatchDetails } from '../api'
import { PlayersTable } from '../components/PlayersTable'

export function MatchPage() {
  const { id = '' } = useParams()
  const [data, setData] = useState<MatchDetails | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    api.getMatch(id).then(setData, (e: Error) => setError(e.message))
  }, [id])

  if (error) return <p className="error">{error}</p>
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

      {match.status === 'failed' && <p className="error">Ошибка обработки: {match.error}</p>}
      {(match.status === 'pending' || match.status === 'parsing') && <p className="muted">Демка ещё обрабатывается.</p>}
      {match.status === 'done' && (
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
