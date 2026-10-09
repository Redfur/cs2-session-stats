import { LoaderCircle } from 'lucide-react'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Link, type LinkProps } from 'react-router'
import { buttonClass, type StyleProps } from './buttonClass'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  StyleProps & {
    // идёт запрос: спиннер вместо иконки, кнопка неактивна
    loading?: boolean
    leading?: ReactNode
  }

export function Button({
  variant,
  size,
  icon,
  loading = false,
  leading,
  className,
  children,
  disabled,
  type = 'button',
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={buttonClass({ variant, size, icon }, className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <LoaderCircle className="animate-spin" aria-hidden /> : leading}
      {icon && loading ? null : children}
    </button>
  )
}

export function ButtonLink({ variant, size, icon, className, ...rest }: LinkProps & StyleProps) {
  return <Link className={buttonClass({ variant, size, icon }, className)} {...rest} />
}
