import { RefreshCw, Users } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { Navigate, useLocation, useNavigate, useParams } from 'react-router'
import { api, ApiError } from '../api'
import { MAX_GROUP_NAME } from '../components/NewGroupDialog'
import { Alert } from '../components/ui/Alert'
import { Button } from '../components/ui/Button'
import { Field, Input } from '../components/ui/Field'
import { paths, pickGroup, useGroups } from '../groups'

// GroupRedirectPage — адреса без группы (главная и /players…): переход в последнюю открытую
// группу, без неё — в первую; если групп нет, экран «Создайте группу».
export function GroupRedirectPage({ players = false }: { players?: boolean }) {
  const { groups, error, reload } = useGroups()
  const rest = useParams()['*'] ?? ''
  const { search } = useLocation()
  if (groups === null) {
    return error ? <LoadError onRetry={() => reload().catch(() => {})} /> : <LoadingCard />
  }
  const g = pickGroup(groups)
  if (!g) return <CreateGroupScreen />
  const to = players ? paths.players(g.id) + (rest ? `/${rest}` : '') : paths.sessions(g.id)
  return <Navigate replace to={{ pathname: to, search }} />
}

// Solo — одиночная карточка по центру страницы.
function Solo({ children, busy, label }: { children: ReactNode; busy?: boolean; label?: string }) {
  return (
    <div className="flex flex-col items-center px-4 pt-[88px] pb-[72px] max-md:pt-10">
      <div
        className="flex w-full max-w-[440px] flex-col gap-5 rounded-xl border border-border bg-surface p-8 max-md:px-5 max-md:py-6"
        aria-busy={busy || undefined}
        aria-label={label}
      >
        {children}
      </div>
    </div>
  )
}

function LoadingCard() {
  const skel = 'block animate-pulse rounded-md bg-surface-2'
  return (
    <Solo busy label="Загружаем группы">
      <div className="flex flex-col items-center gap-3">
        <span className={`${skel} size-12 rounded-lg`} />
        <span className={`${skel} h-6 w-[220px]`} />
        <span className={`${skel} h-3.5 w-[300px] max-w-full`} />
      </div>
      <span className={`${skel} h-9 w-full`} />
      <span className={`${skel} h-11 w-full`} />
    </Solo>
  )
}

function LoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <Solo>
      <Alert tone="error" title="Не удалось загрузить группы.">
        Сервер не ответил. Попробуйте ещё раз.
      </Alert>
      <Button variant="secondary" size="lg" className="w-full" leading={<RefreshCw aria-hidden />} onClick={onRetry}>
        Повторить
      </Button>
    </Solo>
  )
}

function CreateGroupScreen() {
  const [name, setName] = useState('')
  const [tried, setTried] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const { reload } = useGroups()
  const navigate = useNavigate()
  const empty = tried && !name.trim()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setTried(true)
    if (!name.trim()) return
    setBusy(true)
    setError('')
    try {
      const g = await api.createGroup(name)
      // сначала переход: обновлённый список иначе снова сработает перенаправлением этой страницы
      navigate(paths.sessions(g.id))
      reload().catch(() => {})
    } catch (err) {
      // ответ сервера показываем как есть, обрыв соединения — словами макета
      setError(err instanceof ApiError ? err.message : 'Сервер не ответил — попробуйте ещё раз.')
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate>
      <Solo>
        <div className="flex flex-col items-center gap-3 text-center">
          <span className="grid size-12 place-items-center rounded-lg bg-surface-2 text-fg-muted [&>svg]:size-[22px]" aria-hidden>
            <Users />
          </span>
          <h1 className="m-0 text-[24px] leading-[30px] font-bold tracking-[-0.01em] md:text-page">Создайте группу</h1>
          <p className="m-0 max-w-[360px] text-fg-muted">
            Групп пока нет. Создайте группу для своей компании: в ней будут свои сессии, игроки и рейтинг.
          </p>
        </div>
        <Field
          label="Название"
          htmlFor="cg-name"
          error={empty && 'Введите название группы'}
          help="Его видно в переключателе групп. До 100 символов."
        >
          <Input
            id="cg-name"
            type="text"
            maxLength={MAX_GROUP_NAME}
            placeholder="Например, «Основная»"
            autoFocus
            value={name}
            disabled={busy}
            aria-invalid={empty || undefined}
            onChange={(e) => {
              setName(e.target.value)
              setTried(false)
            }}
          />
        </Field>
        {error && (
          <Alert tone="error" title="Не удалось создать группу.">
            {error}
          </Alert>
        )}
        <Button type="submit" variant="primary" size="lg" className="w-full" loading={busy}>
          Создать группу
        </Button>
        <p className="m-0 text-center text-small text-fg-muted">Дальше: новая сессия → загрузка демок → игроки появятся из демок.</p>
      </Solo>
    </form>
  )
}
