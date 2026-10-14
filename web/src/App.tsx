import { Route, Routes } from 'react-router'
import { AppShell } from './components/AppShell'
import { GroupPage } from './pages/GroupPage'
import { GroupRedirectPage } from './pages/GroupRedirectPage'
import { MatchPage } from './pages/MatchPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { PlayerPage } from './pages/PlayerPage'
import { PlayersPage } from './pages/PlayersPage'
import { SessionPage } from './pages/SessionPage'
import { SessionsPage } from './pages/SessionsPage'

export default function App() {
  return (
    <AppShell>
      <Routes>
        <Route path="/g/:gid" element={<SessionsPage />} />
        <Route path="/g/:gid/sessions/:id" element={<SessionPage />} />
        <Route path="/g/:gid/matches/:id" element={<MatchPage />} />
        <Route path="/g/:gid/players" element={<PlayersPage />} />
        <Route path="/g/:gid/players/:steamId" element={<PlayerPage />} />
        <Route path="/g/:gid/players/:steamId/:tab" element={<PlayerPage />} />
        <Route path="/g/:gid/group" element={<GroupPage />} />
        {/* адреса без группы: сессия и матч сами переходят в группу своей сессии */}
        <Route path="/" element={<GroupRedirectPage />} />
        <Route path="/players/*" element={<GroupRedirectPage players />} />
        <Route path="/sessions/:id" element={<SessionPage />} />
        <Route path="/matches/:id" element={<MatchPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </AppShell>
  )
}
