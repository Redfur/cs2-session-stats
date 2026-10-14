import type { HTMLAttributes, ReactNode } from 'react'
import { cx } from './cx'

// danger — карточка необратимых действий («Опасная зона»): красноватая рамка.
export function Card({
  className,
  as: Tag = 'section',
  danger = false,
  ...rest
}: HTMLAttributes<HTMLElement> & { as?: 'section' | 'div' | 'aside'; danger?: boolean }) {
  return (
    <Tag
      className={cx('overflow-hidden rounded-lg border bg-surface', danger ? 'border-metric-bad/30' : 'border-border', className)}
      {...rest}
    />
  )
}

interface CardHeaderProps {
  title: ReactNode
  titleId?: string
  // мелкий текст рядом с заголовком: число строк, пояснение
  note?: ReactNode
  // правая часть: действия или подсказка
  aside?: ReactNode
  className?: string
}

export function CardHeader({ title, titleId, note, aside, className }: CardHeaderProps) {
  return (
    <div
      className={cx(
        'flex min-h-14 flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-border px-4 py-3 md:px-5',
        className,
      )}
    >
      <div className="flex flex-wrap items-baseline gap-3">
        <h2 id={titleId} className="m-0 text-section">
          {title}
        </h2>
        {note != null && note !== '' && <span className="text-small text-fg-muted tabular-nums">{note}</span>}
      </div>
      {aside}
    </div>
  )
}

export function CardBody({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cx('px-4 py-5 md:px-5', className)} {...rest} />
}

export function CardFooter({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cx('border-t border-border px-4 py-3.5 md:px-5', className)} {...rest} />
}
