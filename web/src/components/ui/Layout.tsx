import { Crosshair } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { cx } from './cx'

export interface NavItem {
  to: string
  label: string
  active: boolean
}

// Wrap — колонка страницы шириной до 1344 px с полями 32 (16 на узком экране).
export function Wrap({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cx('mx-auto w-full max-w-[1344px] px-4 md:px-8', className)}>{children}</div>
}

interface LayoutProps {
  // куда ведёт лого; без адреса лого не ссылка (экран «Создайте группу»)
  logoTo?: string
  // переключатель группы справа от лого
  switcher?: ReactNode
  nav?: NavItem[]
  children: ReactNode
}

// Layout — шапка «лого / группа» с навигацией и содержимое страницы.
// На узком экране навигация переносится второй строкой, переключатель остаётся в первой.
export function Layout({ logoTo, switcher, nav, children }: LayoutProps) {
  const logoClass =
    'focus-ring inline-flex shrink-0 items-center gap-2.5 whitespace-nowrap text-[15px] font-extrabold tracking-[-0.01em] text-fg hover:text-fg hover:no-underline'
  const logo = (
    <>
      <span className="grid size-7 place-items-center rounded-md bg-accent text-accent-fg">
        <Crosshair className="size-[18px]" strokeWidth={2.25} aria-hidden />
      </span>
      <span className="max-md:sr-only">CS2 Session Stats</span>
    </>
  )
  return (
    <div className="min-h-screen overflow-x-clip bg-bg">
      <header className="border-b border-border bg-topbar">
        <Wrap className="flex flex-wrap items-center gap-x-2 md:h-14 md:flex-nowrap md:gap-x-10">
          <div className="flex h-14 min-w-0 flex-1 items-center gap-2 md:flex-none md:gap-2.5">
            {logoTo ? (
              <Link to={logoTo} className={logoClass} aria-label="CS2 Session Stats — к сессиям группы">
                {logo}
              </Link>
            ) : (
              <span className={logoClass}>{logo}</span>
            )}
            {switcher && (
              <>
                <span className="shrink-0 text-[22px] leading-6 font-light text-border-hover max-md:hidden" aria-hidden>
                  /
                </span>
                {switcher}
              </>
            )}
          </div>
          {nav && (
            <nav
              className="order-3 -mx-4 flex h-11 basis-[calc(100%+32px)] items-stretch gap-5 overflow-x-auto border-t border-border px-4 [scrollbar-width:none] md:order-none md:mx-0 md:h-14 md:basis-auto md:gap-6 md:overflow-visible md:border-t-0 md:px-0"
              aria-label="Основная навигация"
            >
              {nav.map((n) => (
                <Link
                  key={n.label}
                  to={n.to}
                  aria-current={n.active ? 'page' : undefined}
                  className={cx(
                    'focus-ring flex items-center rounded-none border-b-2 pt-0.5 font-semibold whitespace-nowrap hover:no-underline',
                    n.active ? 'border-accent text-fg hover:text-fg' : 'border-transparent text-fg-muted hover:text-fg',
                  )}
                >
                  {n.label}
                </Link>
              ))}
            </nav>
          )}
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
