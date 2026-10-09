import type { ReactNode } from 'react'
import { cx } from './cx'

interface EmptyStateProps {
  icon: ReactNode
  title: ReactNode
  text?: ReactNode
  actions?: ReactNode
  tone?: 'neutral' | 'error'
  // заголовок уровня страницы (ошибка загрузки) или секции
  as?: 'h1' | 'h2' | 'h3'
  className?: string
}

// EmptyState объясняет, почему пусто, и даёт следующий шаг.
export function EmptyState({ icon, title, text, actions, tone = 'neutral', as: H = 'h3', className }: EmptyStateProps) {
  return (
    <div className={cx('flex flex-col items-center gap-2 px-6 py-10 text-center', className)}>
      <span
        className={cx(
          'mb-1.5 grid size-12 place-items-center rounded-lg [&>svg]:size-[22px]',
          tone === 'error' ? 'bg-status-error/10 text-status-error' : 'bg-surface-2 text-fg-muted',
        )}
        aria-hidden
      >
        {icon}
      </span>
      <H className={cx('m-0', H === 'h3' ? 'text-sub' : 'text-section')}>{title}</H>
      {text && <p className="m-0 max-w-[380px] text-small text-fg-muted">{text}</p>}
      {actions && <div className="mt-2 flex flex-wrap justify-center gap-2">{actions}</div>}
    </div>
  )
}
