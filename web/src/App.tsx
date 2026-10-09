import { Route, Routes } from 'react-router'
import { Layout } from './components/ui/Layout'
import { MatchPage } from './pages/MatchPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { PlayerPage } from './pages/PlayerPage'
import { PlayersPage } from './pages/PlayersPage'
import { SessionPage } from './pages/SessionPage'
import { SessionsPage } from './pages/SessionsPage'

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<SessionsPage />} />
        <Route path="/sessions/:id" element={<SessionPage />} />
        <Route path="/matches/:id" element={<MatchPage />} />
        <Route path="/players" element={<PlayersPage />} />
        <Route path="/players/:steamId" element={<PlayerPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Layout>
  )
}
