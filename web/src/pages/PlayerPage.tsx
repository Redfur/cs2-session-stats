import { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router'
import { api, type PlayerProfile } from '../api'
import { PeriodFilter } from '../components/PeriodFilter'
import { PlayersTable } from '../components/PlayersTable'

const resultLabel = { win: 'победа', loss: 'поражение', draw: 'ничья' } as const

export function PlayerPage() {
  const { steamId = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const [data, setData] = useState<PlayerProfile | null>(null)
  const [error, setError] = useState('')
  const query = params.toString()

  useEffect(() => {
    let stale = false
    api.getPlayer(steamId, query).then(
      (d) => {
        if (stale) return
        setData(d)
        setError('')
      },
      (e: Error) => !stale && setError(e.message),
    )
    return () => {
      stale = true
    }
  }, [steamId, query])

  if (error && !data) return <p className="error">{error}</p>
  if (!data) return <p className="muted">Загрузка…</p>

  return (
    <>
      <h2>{data.name}</h2>
      <p>
        <a href={`https://steamcommunity.com/profiles/${data.steamId}`} target="_blank" rel="noreferrer">
          профиль Steam
        </a>{' '}
        · <Link to="/players">все игроки</Link>
      </p>
      <PeriodFilter params={params} setParams={setParams} />
      {error && <p className="error">{error}</p>}

      <h3>Итоги</h3>
      <PlayersTable players={[data.totals]} aggregate />

      <h3>По сессиям</h3>
      {data.sessions.length === 0 ? (
        <p className="muted">Нет данных.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Сессия</th>
                <th>Дата</th>
                <th title="Матчей">М</th>
                <th title="Побед">П</th>
                <th>K/D</th>
                <th>ADR</th>
                <th>KAST</th>
                <th>Rating</th>
              </tr>
            </thead>
            <tbody>
              {data.sessions.map((s) => (
                <tr key={s.session.id}>
                  <td>
                    <Link to={`/sessions/${s.session.id}`}>{s.session.title || s.session.date}</Link>
                  </td>
                  <td>{s.session.date}</td>
                  <td>{s.matches ?? 0}</td>
                  <td>{s.wins ?? 0}</td>
                  <td>{s.kd.toFixed(2)}</td>
                  <td>{s.adr.toFixed(1)}</td>
                  <td>{s.kastPct.toFixed(0)}%</td>
                  <td>
                    <b>{s.rating.toFixed(2)}</b>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <h3>По картам</h3>
      {data.maps.length === 0 ? (
        <p className="muted">Нет данных.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Карта</th>
                <th title="Матчей">М</th>
                <th title="Побед">П</th>
                <th>K/D</th>
                <th>ADR</th>
                <th>Rating</th>
              </tr>
            </thead>
            <tbody>
              {data.maps.map((m) => (
                <tr key={m.map}>
                  <td>{m.map || '—'}</td>
                  <td>{m.matches ?? 0}</td>
                  <td>{m.wins ?? 0}</td>
                  <td>{m.kd.toFixed(2)}</td>
                  <td>{m.adr.toFixed(1)}</td>
                  <td>
                    <b>{m.rating.toFixed(2)}</b>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <h3>Матчи</h3>
      {data.matches.length === 0 ? (
        <p className="muted">Нет данных.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Сессия</th>
                <th>#</th>
                <th>Карта</th>
                <th title="Счёт команды игрока : счёт соперника">Счёт</th>
                <th>Исход</th>
                <th>K-D-A</th>
                <th>ADR</th>
                <th>Rating</th>
              </tr>
            </thead>
            <tbody>
              {data.matches.map((m) => {
                const [own, other] = m.team === 'B' ? [m.scoreB, m.scoreA] : [m.scoreA, m.scoreB]
                return (
                  <tr key={m.matchId}>
                    <td>
                      <Link to={`/sessions/${m.sessionId}`}>{m.sessionTitle || m.sessionDate}</Link>
                    </td>
                    <td>
                      <Link to={`/matches/${m.matchId}`}>{m.ordinal}</Link>
                    </td>
                    <td>{m.map || '—'}</td>
                    <td>
                      {own}:{other}
                    </td>
                    <td>{m.result ? resultLabel[m.result] : ''}</td>
                    <td>
                      {m.kills}-{m.deaths}-{m.assists}
                    </td>
                    <td>{m.adr.toFixed(1)}</td>
                    <td>
                      <b>{m.rating.toFixed(2)}</b>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
