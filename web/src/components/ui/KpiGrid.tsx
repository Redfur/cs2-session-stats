import type { CSSProperties, ReactNode } from 'react'
import { cx } from './cx'

// KpiGrid — сетка показателей карточки: ячейки через линию 1 px. На узком экране по две
// в ряд, нечётная последняя — во всю ширину.
export function KpiGrid({ cols, children, className }: { cols: number; children: ReactNode; className?: string }) {
  return (
    <div
      className={cx(
        'grid grid-cols-2 gap-px bg-border md:grid-cols-[repeat(var(--k),minmax(0,1fr))]',
        '[&>*:last-child:nth-child(odd)]:col-span-2 md:[&>*:last-child:nth-child(odd)]:col-span-1',
        className,
      )}
      style={{ '--k': cols } as CSSProperties}
    >
      {children}
    </div>
  )
}

interface KpiProps {
  label: ReactNode
  value: ReactNode
  // знаменатель рядом с числом: «/ 82»
  of?: ReactNode
  sub?: ReactNode
  // чип покрытия под числом
  coverage?: ReactNode
}

export function Kpi({ label, value, of, sub, coverage }: KpiProps) {
  return (
    <div className="flex min-w-0 flex-col items-start gap-1 bg-surface px-4 py-3.5 md:px-5 md:py-4">
      <span className="text-small font-semibold text-fg-muted">{label}</span>
      <span className="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-[22px] leading-7 font-extrabold tracking-[-0.02em] tabular-nums md:text-[26px] md:leading-8">
        {value}
        {of != null && <small className="text-[14px] leading-5 font-semibold tracking-normal text-fg-muted">{of}</small>}
      </span>
      {sub != null && <span className="text-small text-fg-muted">{sub}</span>}
      {coverage != null && <span className="mt-0.5">{coverage}</span>}
    </div>
  )
}
