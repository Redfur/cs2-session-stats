import { weaponInfo } from '../../weapons'
import { cx } from './cx'

// Заглушка неопознанного оружия: рамка с вопросом цветом fg-faint.
function Fallback() {
  return (
    <svg viewBox="0 0 26 20" width={20.8} height={16} className="block flex-none fill-current" aria-hidden>
      <path d="M1 3L25 3L25 17L1 17ZM2.6 15.4L23.4 15.4L23.4 4.6L2.6 4.6ZM10 6.6L16 6.6L16 8.2L10 8.2ZM14.4 6.6L16 6.6L16 10.4L14.4 10.4ZM11.8 9.2L16 9.2L16 10.8L11.8 10.8ZM11.8 9.2L13.4 9.2L13.4 11.8L11.8 11.8ZM11.8 12.6L13.4 12.6L13.4 14.2L11.8 14.2Z" />
    </svg>
  )
}

// WeaponIcon — игровая иконка оружия высотой 16 px; название — в подсказке и для скринридера.
export function WeaponIcon({ code, className }: { code: string; className?: string }) {
  const w = weaponInfo(code)
  const title = w.known ? w.name : `Оружие не опознано: ${code}`
  return (
    <span className={cx('inline-flex flex-none items-center', w.icon ? 'text-fg' : 'text-fg-faint', className)} title={title}>
      {w.icon ? <img src={w.icon} alt="" className="block h-4 w-auto opacity-82" /> : <Fallback />}
      <span className="sr-only">{w.name}</span>
    </span>
  )
}

// WeaponLabel — ячейка таблицы оружия: иконка в колонке 52 px и название.
export function WeaponLabel({ code }: { code: string }) {
  const w = weaponInfo(code)
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <span
        className={cx('flex w-[52px] flex-none items-center', w.known ? 'text-metric-mid' : 'text-fg-faint')}
        title={w.known ? w.name : `В справочнике нет такого оружия: ${code}`}
      >
        {w.icon ? <img src={w.icon} alt="" className="block h-4 w-auto opacity-82" /> : <Fallback />}
      </span>
      <span className="truncate font-semibold">{w.name}</span>
    </span>
  )
}
