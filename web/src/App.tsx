import { Link, Route, Routes } from 'react-router'
import { MatchPage } from './pages/MatchPage'
import { SessionPage } from './pages/SessionPage'
import { SessionsPage } from './pages/SessionsPage'

export default function App() {
  return (
    <>
      <header>
        <h1>
          <Link to="/">CS2 Session Stats</Link>
        </h1>
      </header>
      <Routes>
        <Route path="/" element={<SessionsPage />} />
        <Route path="/sessions/:id" element={<SessionPage />} />
        <Route path="/matches/:id" element={<MatchPage />} />
        <Route path="*" element={<p>Страница не найдена.</p>} />
      </Routes>
    </>
  )
}
