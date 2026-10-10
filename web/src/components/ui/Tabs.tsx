import { NavLink } from 'react-router'
import { cx } from './cx'

interface TabItem {
  to: string
  label: string
  // активна только при точном совпадении адреса (вкладка-родитель)
  end?: boolean
}

// Tabs — разделы страницы с собственными адресами. Активная вкладка подчёркнута акцентом;
// на узком экране ряд прокручивается внутри себя.
export function Tabs({ items, label }: { items: TabItem[]; label: string }) {
  return (
    <nav
      aria-label={label}
      className="-mb-2 flex gap-5 overflow-x-auto border-b border-border [scrollbar-width:none] md:gap-7 [&::-webkit-scrollbar]:hidden"
    >
      {items.map((t) => (
        <NavLink
          key={t.to}
          to={t.to}
          end={t.end}
          className={({ isActive }) =>
            cx(
              'focus-ring -mb-px flex h-11 flex-none items-center rounded-none border-b-2 pt-0.5 text-[14px] font-bold whitespace-nowrap md:text-[15px]',
              'hover:text-fg hover:no-underline',
              isActive ? 'border-accent text-fg' : 'border-transparent text-fg-muted',
            )
          }
        >
          {t.label}
        </NavLink>
      ))}
    </nav>
  )
}
