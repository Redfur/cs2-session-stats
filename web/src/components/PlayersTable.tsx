import { Link } from 'react-router'
import type { PlayerRow } from '../api'

const resultLabel = { win: 'П', loss: 'Пр', draw: 'Н' } as const

// Таблица статистики игроков: для матча (scoreboard) и сводная по нескольким матчам (aggregate: колонки матчей и побед).
export function PlayersTable({ players, aggregate = false }: { players: PlayerRow[]; aggregate?: boolean }) {
  if (players.length === 0) return <p className="muted">Нет данных.</p>
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Игрок</th>
            {aggregate ? (
              <>
                <th title="Матчей">М</th>
                <th title="Побед">П</th>
              </>
            ) : (
              <th title="Исход">Исход</th>
            )}
            <th>K</th>
            <th>D</th>
            <th>A</th>
            <th>K/D</th>
            <th>ADR</th>
            <th>HS%</th>
            <th>KAST</th>
            <th>2K</th>
            <th>3K</th>
            <th>4K</th>
            <th>5K</th>
            <th title="Первые убийства / первые смерти раунда">Open K/D</th>
            <th>Rating</th>
          </tr>
        </thead>
        <tbody>
          {players.map((p) => (
            <tr key={p.steamId}>
              <td>
                <Link to={`/players/${p.steamId}`}>{p.name}</Link>
              </td>
              {aggregate ? (
                <>
                  <td>{p.matches ?? 0}</td>
                  <td>{p.wins ?? 0}</td>
                </>
              ) : (
                <td>{p.result ? resultLabel[p.result] : ''}</td>
              )}
              <td>{p.kills}</td>
              <td>{p.deaths}</td>
              <td>{p.assists}</td>
              <td>{p.kd.toFixed(2)}</td>
              <td>{p.adr.toFixed(1)}</td>
              <td>{p.hsPct.toFixed(0)}%</td>
              <td>{p.kastPct.toFixed(0)}%</td>
              <td>{p.k2}</td>
              <td>{p.k3}</td>
              <td>{p.k4}</td>
              <td>{p.k5}</td>
              <td>
                {p.openingKills}/{p.openingDeaths}
              </td>
              <td>
                <b>{p.rating.toFixed(2)}</b>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
