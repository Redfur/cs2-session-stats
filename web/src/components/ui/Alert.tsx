import { CircleAlert, RefreshCw } from 'lucide-react'
import type { ReactNode } from 'react'
import { cx } from './cx'

interface AlertProps {
  tone: 'info' | 'error'
  // жирное начало сообщения
  title?: ReactNode
  children?: ReactNode
  // кнопка справа: повторить, сбросить
  action?: ReactNode
  // иконка вместо стандартной (например, крутящаяся для процесса)
  icon?: ReactNode
  className?: string
}

// Alert — сообщение там, где возникла проблема, или плашка идущего процесса.
export function Alert({ tone, title, children, action, icon, className }: AlertProps) {
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={cx(
        'flex flex-wrap items-start gap-3 rounded-md border px-3.5 py-3 text-[13px] leading-[19px] md:flex-nowrap',
        tone === 'error'
          ? 'border-status-error/30 bg-status-error/12'
          : 'border-status-progress/28 bg-status-progress/10',
        className,
      )}
    >
      <span className={cx('mt-0.5 flex-none [&>svg]:size-4', tone === 'error' ? 'text-status-error' : 'text-status-progress')}>
        {icon ?? (tone === 'error' ? <CircleAlert aria-hidden /> : <RefreshCw className="animate-spin" aria-hidden />)}
      </span>
      <div className="min-w-0 flex-1">
        {title && <b className="font-bold">{title}</b>} {children}
      </div>
      {action && <div className="ml-auto flex-none">{action}</div>}
    </div>
  )
}
