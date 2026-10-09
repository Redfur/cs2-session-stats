// Типы и вызовы JSON API бэкенда (internal/api).

export type MatchStatus = 'pending' | 'parsing' | 'done' | 'failed'

export interface Session {
  id: number
  date: string
  title: string
  createdAt: string
}

export interface SessionSummary extends Session {
  matchCount: number
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
  getMatch: (id: string) => request<MatchDetails>('GET', `/api/matches/${id}`),

  // XHR вместо fetch: у fetch нет прогресса отправки, а демки весят сотни мегабайт.
  uploadDemos(sessionId: string, files: File[], onProgress: (loaded: number, total: number) => void) {
    return new Promise<UploadResult[]>((resolve, reject) => {
      const form = new FormData()
      for (const f of files) form.append('files', f)
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/api/sessions/${sessionId}/demos`)
      xhr.upload.onprogress = (e) => onProgress(e.loaded, e.total)
      xhr.onload = () => {
        let data: unknown = null
        try {
          data = JSON.parse(xhr.responseText)
        } catch {
          /* ответ не JSON */
        }
        if (xhr.status >= 200 && xhr.status < 300) resolve(data as UploadResult[])
        else reject(new Error((data as { error?: string } | null)?.error ?? `HTTP ${xhr.status}`))
      }
      xhr.onerror = () => reject(new Error('сетевая ошибка'))
      xhr.send(form)
    })
  },
}

export function todayISO(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export function sessionTitle(s: Session): string {
  return s.title || s.date
}
