import { Crosshair } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link, useLocation } from 'react-router'
import { cx } from './cx'

const NAV = [
  // «Сессии» активна и на страницах сессии и матча
  { to: '/', label: 'Сессии', match: (p: string) => p === '/' || p.startsWith('/sessions/') || p.startsWith('/matches/') },
  { to: '/players', label: 'Игроки', match: (p: string) => p === '/players' || p.startsWith('/players/') },
]

// Wrap — колонка страницы шириной до 1344 px с полями 32 (16 на узком экране).
export function Wrap({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cx('mx-auto w-full max-w-[1344px] px-4 md:px-8', className)}>{children}</div>
}

export function Layout({ children }: { children: ReactNode }) {
  const { pathname } = useLocation()
  return (
    <div className="min-h-screen overflow-x-clip bg-bg">
      <header className="border-b border-border bg-topbar">
        <Wrap className="flex h-14 items-center gap-5 md:gap-10">
          <Link
            to="/"
            className="focus-ring inline-flex items-center gap-2.5 whitespace-nowrap text-[14px] font-extrabold tracking-[-0.01em] text-fg hover:text-fg hover:no-underline md:text-[15px]"
          >
            <span className="grid size-7 place-items-center rounded-md bg-accent text-accent-fg">
              <Crosshair className="size-[18px]" strokeWidth={2.25} aria-hidden />
            </span>
            CS2 Session Stats
          </Link>
          <nav className="flex h-14 items-stretch gap-4 md:gap-6" aria-label="Основная навигация">
            {NAV.map((n) => {
              const active = n.match(pathname)
              return (
                <Link
                  key={n.to}
                  to={n.to}
                  aria-current={active ? 'page' : undefined}
                  className={cx(
                    'focus-ring flex items-center rounded-none border-b-2 pt-0.5 font-semibold hover:no-underline',
                    active ? 'border-accent text-fg hover:text-fg' : 'border-transparent text-fg-muted hover:text-fg',
                  )}
                >
                  {n.label}
                </Link>
              )
            })}
          </nav>
        </Wrap>
      </header>
      <main>{children}</main>
    </div>
  )
}

// Page — вертикальный поток секций страницы.
export function Page({ className, children }: { className?: string; children: ReactNode }) {
  return <Wrap className={cx('flex flex-col gap-6 pt-6 pb-[72px] md:gap-8 md:pt-10', className)}>{children}</Wrap>
}
