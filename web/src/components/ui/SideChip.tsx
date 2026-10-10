import type { Side } from '../../api'
import { cx } from './cx'

// SideChip — сторона в раунде с подписью: CT синий, T оранжевый.
export function SideChip({ side, className }: { side: Side; className?: string }) {
  return (
    <span
      className={cx(
        'inline-flex h-[18px] min-w-[26px] flex-none items-center justify-center rounded-[4px] px-[5px] text-over font-extrabold tracking-[0.02em] whitespace-nowrap',
        side === 'CT' ? 'bg-side-ct/15 text-side-ct' : 'bg-side-t/15 text-side-t',
        className,
      )}
    >
      {side}
    </span>
  )
}

// SideSwatch — квадрат цвета стороны; только там, где сторона уже названа рядом.
export function SideSwatch({ side }: { side: Side }) {
  return <span className={cx('size-2 flex-none rounded-[2px]', side === 'CT' ? 'bg-side-ct' : 'bg-side-t')} aria-hidden />
}
