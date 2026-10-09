import { ArrowLeft } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

interface PageHeaderProps {
  back?: { to: string; label: ReactNode }
  // заголовок страницы; titleSlot заменяет его целиком (например, полем правки)
  title?: ReactNode
  titleSlot?: ReactNode
  meta?: ReactNode
  actions?: ReactNode
}

// Заголовок страницы: ссылка назад, заголовок, мета-строка и действия справа.
// На узком экране действия переносятся под заголовок.
export function PageHeader({ back, title, titleSlot, meta, actions }: PageHeaderProps) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-4">
      <div className="min-w-0">
        {back && (
          <Link to={back.to} className="focus-ring mb-2.5 inline-flex items-center gap-1.5 text-[13px] font-semibold">
            <ArrowLeft className="size-4" aria-hidden />
            {back.label}
          </Link>
        )}
        <div className="flex min-h-[42px] items-center gap-1.5">
          {titleSlot ?? <h1 className="m-0 text-[24px] leading-[30px] font-bold tracking-[-0.01em] md:text-page">{title}</h1>}
        </div>
        {meta && <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-fg-muted">{meta}</div>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}

// MetaItem — элемент мета-строки заголовка, иконка и текст.
export function MetaItem({ children, className }: { children: ReactNode; className?: string }) {
  return <span className={'inline-flex items-center gap-1.5 [&>svg]:size-4 ' + (className ?? '')}>{children}</span>
}
