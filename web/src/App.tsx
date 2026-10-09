import { Link, Route, Routes } from 'react-router'
import { MatchPage } from './pages/MatchPage'
import { PlayerPage } from './pages/PlayerPage'
import { PlayersPage } from './pages/PlayersPage'
import { SessionPage } from './pages/SessionPage'
import { SessionsPage } from './pages/SessionsPage'

export default function App() {
  return (
    <>
      <header>
        <h1>
          <Link to="/">CS2 Session Stats</Link>
        </h1>
        <nav>
          <Link to="/">Сессии</Link> | <Link to="/players">Игроки</Link>
        </nav>
      </header>
      <Routes>
        <Route path="/" element={<SessionsPage />} />
        <Route path="/sessions/:id" element={<SessionPage />} />
        <Route path="/matches/:id" element={<MatchPage />} />
        <Route path="/players" element={<PlayersPage />} />
        <Route path="/players/:steamId" element={<PlayerPage />} />
        <Route path="*" element={<p>Страница не найдена.</p>} />
      </Routes>
    </>
  )
}
