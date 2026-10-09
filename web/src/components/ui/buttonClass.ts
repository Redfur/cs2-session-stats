import { cx } from './cx'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'danger-quiet' | 'delete'
export type ButtonSize = 'md' | 'sm' | 'xs'

const VARIANTS: Record<ButtonVariant, string> = {
  primary: 'bg-primary text-primary-fg enabled:hover:bg-primary-hover hover:text-primary-fg',
  secondary:
    'bg-surface-2 border-border-strong text-fg enabled:hover:bg-surface-hover enabled:hover:border-border-hover hover:text-fg',
  ghost: 'bg-transparent text-fg-muted enabled:hover:bg-surface-hover enabled:hover:text-fg',
  danger: 'bg-danger text-white enabled:hover:bg-danger-hover',
  'danger-quiet':
    'bg-transparent border-metric-bad/35 text-metric-bad enabled:hover:bg-metric-bad/14 enabled:hover:border-metric-bad/60',
  // тихое удаление в строке таблицы: серое, красное при наведении
  delete: 'bg-transparent text-fg-muted enabled:hover:bg-metric-bad/14 enabled:hover:text-metric-bad',
}

const SIZES: Record<ButtonSize, { text: string; icon: string }> = {
  md: { text: 'h-9 px-3.5 gap-2 text-[14px] [&_svg]:size-4', icon: 'size-9 [&_svg]:size-4' },
  sm: { text: 'h-[30px] px-2.5 gap-1.5 text-[13px] [&_svg]:size-3.5', icon: 'size-[30px] [&_svg]:size-3.5' },
  xs: { text: 'h-[26px] px-2 gap-1 text-[12px] [&_svg]:size-3.5', icon: 'size-[26px] rounded-sm [&_svg]:size-3.5' },
}

export interface StyleProps {
  variant?: ButtonVariant
  size?: ButtonSize
  // кнопка-иконка без подписи: квадратная, подпись — в aria-label
  icon?: boolean
}

export function buttonClass({ variant = 'secondary', size = 'md', icon = false }: StyleProps, extra?: string): string {
  return cx(
    'focus-ring inline-flex shrink-0 cursor-pointer items-center justify-center whitespace-nowrap rounded-md border border-transparent font-semibold leading-5 no-underline transition-colors hover:no-underline',
    'disabled:cursor-not-allowed disabled:opacity-40 aria-busy:cursor-progress aria-busy:opacity-100',
    VARIANTS[variant],
    icon ? SIZES[size].icon : SIZES[size].text,
    extra,
  )
}
