// Типы и вызовы JSON API бэкенда (internal/api).

import { formatDateLong } from './format'

export type MatchStatus = 'pending' | 'parsing' | 'done' | 'failed'

export interface Session {
  id: number
  date: string
  title: string
  createdAt: string
}

export interface SessionSummary extends Session {
  matchCount: number
  // матчи в статусе failed
  failedCount: number
  // карты матчей с результатом в порядке номеров, без повторов
  maps: string[]
  // игрок с наибольшим rating в итогах сессии; нет, если результатов нет
  best?: { steamId: string; name: string; rating: number }
}

export interface Match {
  id: number
  sessionId: number
  ordinal: number
  sha256: string
  originalName: string
  status: MatchStatus
  error?: string
  map: string
  rounds: number
  scoreA: number
  scoreB: number
  createdAt: string
  parsedAt?: string
  // есть посчитанный результат; сохраняется во время пересчёта и при его ошибке
  hasResult: boolean
  processedVersion: number
}

export interface PlayerRow {
  steamId: string
  name: string
  team?: 'A' | 'B'
  result?: 'win' | 'loss' | 'draw'
  matches?: number
  wins?: number
  rounds: number
  kills: number
  deaths: number
  assists: number
  hsKills: number
  damage: number
  kastRounds: number
  k1: number
  k2: number
  k3: number
  k4: number
  k5: number
  openingKills: number
  openingDeaths: number
  kd: number
  adr: number
  hsPct: number
  kastPct: number
  rating: number
}

export interface SessionDetails {
  session: Session
  matches: Match[]
  players: PlayerRow[]
}

export interface MatchDetails {
  match: Match
  players: PlayerRow[]
}

export interface PlayersResponse {
  players: PlayerRow[]
}

export interface ProfileSession extends PlayerRow {
  session: Session
}

export interface ProfileMap extends PlayerRow {
  map: string
}

export interface ProfileMatch extends PlayerRow {
  matchId: number
  sessionId: number
  sessionDate: string
  sessionTitle: string
  ordinal: number
  map: string
  scoreA: number
  scoreB: number
}

export interface PlayerProfile {
  steamId: string
  name: string
  totals: PlayerRow
  sessions: ProfileSession[]
  maps: ProfileMap[]
  matches: ProfileMatch[]
}

// Состояние расчёта дуэлей выборки: нет матчей, ни у одного матча нет дуэлей, не у всех, у всех.
export type DuelsStatus = 'no_matches' | 'unavailable' | 'partial' | 'complete'

export interface DuelPlayer {
  steamId: string
  name: string
  team?: 'A' | 'B'
}

// Личный счёт пары: kills — убийства killer против victim, deaths — обратные,
// share — доля kills / (kills + deaths), null при 0:0; maps — матчей, где пара была соперниками.
export interface DuelCell {
  killerId: string
  victimId: string
  kills: number
  deaths: number
  share: number | null
  maps: number
}

export interface MatchDuels {
  status: 'complete' | 'unavailable'
  players: DuelPlayer[]
  cells: DuelCell[]
}

export interface SessionDuels {
  status: DuelsStatus
  eligibleMatches: number
  coveredMatches: number
  // матчи с результатом, обработанные до появления дуэлей
  uncoveredMatches: { id: number; ordinal: number }[]
  players: DuelPlayer[]
  cells: DuelCell[]
}

export interface Opponent {
  steamId: string
  name: string
  kills: number
  deaths: number
  share: number | null
  maps: number
}

export interface PlayerDuels {
  status: DuelsStatus
  eligibleMatches: number
  coveredMatches: number
  coveredSessions: number
  mostKilled: Opponent[]
  mostKilledBy: Opponent[]
  opponents: Opponent[]
}

export interface UploadResult {
  fileName: string
  status: 'accepted' | 'duplicate' | 'error'
  matchId?: number
  sessionId?: number
  error?: string
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
  const resp = await fetch(url, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await resp.json().catch(() => null)
  if (!resp.ok) {
    throw new Error(data?.error ?? `HTTP ${resp.status}`)
  }
  return data as T
}

export const api = {
  listSessions: () => request<SessionSummary[]>('GET', '/api/sessions'),
  createSession: (date: string, title: string) =>
    request<Session>('POST', '/api/sessions', { date, title }),
  getSession: (id: string) => request<SessionDetails>('GET', `/api/sessions/${id}`),
  updateSession: (id: string, date: string, title: string) =>
    request<Session>('PATCH', `/api/sessions/${id}`, { date, title }),
  // удаляет сессию с матчами и файлами демок; ответ 204 без тела
  deleteSession: (id: string) => request<void>('DELETE', `/api/sessions/${id}`),
  // matchIds — все матчи сессии в новом порядке
  reorderMatches: (sessionId: string, matchIds: number[]) =>
    request<Match[]>('PUT', `/api/sessions/${sessionId}/order`, { matchIds }),
  deleteMatch: (id: string | number) => request<void>('DELETE', `/api/matches/${id}`),
  getMatch: (id: string) => request<MatchDetails>('GET', `/api/matches/${id}`),
  reparseMatch: (id: string | number) => request<Match>('POST', `/api/matches/${id}/reparse`),
  reparseSession: (id: string) => request<{ queued: number }>('POST', `/api/sessions/${id}/reparse`),
  // query — строка параметров фильтра без «?» (from, to, session, player, together, minMatches)
  listPlayers: (query: string) => request<PlayersResponse>('GET', `/api/players?${query}`),
  getPlayer: (steamId: string, query: string) => request<PlayerProfile>('GET', `/api/players/${steamId}?${query}`),
  getMatchDuels: (id: string | number) => request<MatchDuels>('GET', `/api/matches/${id}/duels`),
  getSessionDuels: (id: string | number) => request<SessionDuels>('GET', `/api/sessions/${id}/duels`),
  getPlayerDuels: (steamId: string, query: string) => request<PlayerDuels>('GET', `/api/players/${steamId}/duels?${query}`),

  // Отправляет один файл. XHR вместо fetch: у fetch нет прогресса отправки, а демки весят сотни мегабайт.
  // При signal.abort() отправка обрывается, промис отклоняется с DOMException AbortError.
  uploadDemo(
    sessionId: string,
    file: File,
    onProgress: (loaded: number, total: number) => void,
    signal?: AbortSignal,
  ): Promise<UploadResult> {
    return new Promise<UploadResult>((resolve, reject) => {
      if (signal?.aborted) {
        reject(new DOMException('загрузка отменена', 'AbortError'))
        return
      }
      const form = new FormData()
      form.append('files', file)
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/api/sessions/${sessionId}/demos`)
      xhr.upload.onprogress = (e) => onProgress(e.loaded, e.total)
      xhr.onload = () => {
        signal?.removeEventListener('abort', abort)
        let data: unknown = null
        try {
          data = JSON.parse(xhr.responseText)
        } catch {
          /* ответ не JSON */
        }
        const list = data as UploadResult[] | null
        if (xhr.status >= 200 && xhr.status < 300 && Array.isArray(list) && list.length > 0) resolve(list[0])
        else reject(new Error((data as { error?: string } | null)?.error ?? `HTTP ${xhr.status}`))
      }
      xhr.onerror = () => {
        signal?.removeEventListener('abort', abort)
        reject(new Error('соединение оборвалось, файл не сохранён'))
      }
      xhr.onabort = () => reject(new DOMException('загрузка отменена', 'AbortError'))
      const abort = () => xhr.abort()
      signal?.addEventListener('abort', abort, { once: true })
      xhr.send(form)
    })
  },
}

export function isProcessing(m: Match): boolean {
  return m.status === 'pending' || m.status === 'parsing'
}

export function todayISO(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function sessionTitle(s: Session): string {
  return s.title || formatDateLong(s.date)
}
