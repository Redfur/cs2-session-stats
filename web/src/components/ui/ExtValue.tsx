import type { ReactNode } from 'react'
import { cx } from './cx'

// Состояния значения расширенной статистики. Известный ноль — обычное число; «не посчитано»
// и «проверяется» отличаются формой, а не только цветом. Причина всегда в подсказке.

// NotComputed — прочерк вместо значения, которое не посчитано; reason — почему.
export function NotComputed({ reason }: { reason: string }) {
  return (
    <span
      title={reason}
      className="cursor-help font-medium text-fg-faint underline decoration-dotted decoration-1 underline-offset-4"
    >
      —<span className="sr-only"> ({reason})</span>
    </span>
  )
}

// Checking — метрика ещё проверяется: нейтральная плашка, не ошибка.
export function Checking({ title }: { title: string }) {
  return (
    <span
      title={title}
      className="inline-flex h-[22px] cursor-help items-center self-center whitespace-nowrap rounded-full bg-status-neutral/13 px-[9px] text-small font-semibold tracking-normal text-status-neutral"
    >
      проверяется
    </span>
  )
}

// CoverageChip — по скольким картам посчитано значение, когда покрытие у показателей разное.
export function CoverageChip({ children, title }: { children: ReactNode; title?: string }) {
  return (
    <span
      title={title}
      className="inline-flex h-[18px] flex-none cursor-help items-center whitespace-nowrap rounded-[4px] px-[5px] text-over font-semibold tracking-normal text-fg-muted shadow-[inset_0_0_0_1px_var(--color-border-strong)]"
    >
      {children}
    </span>
  )
}

// ShareBar — нейтральная полоса доли 0..1 рядом с числом. Это не шкала: цвет у неё один.
export function ShareBar({ share, className }: { share: number; className?: string }) {
  return (
    <span
      className={cx('ml-2.5 inline-block h-1 w-14 flex-none overflow-hidden rounded-[2px] bg-surface-hover align-middle', className)}
      aria-hidden
    >
      <span className="block h-full rounded-[2px] bg-status-neutral" style={{ width: `${Math.round(share * 100)}%` }} />
    </span>
  )
}
