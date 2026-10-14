import { Menu, MenuButton, MenuItem, MenuItems, MenuSeparator } from '@headlessui/react'
import { Check, ChevronsUpDown, Plus } from 'lucide-react'
import { Link } from 'react-router'
import { plural } from '../../format'
import { cx } from './cx'

interface SwitcherGroup {
  id: number
  name: string
  sessionCount: number
}

interface GroupSwitcherProps {
  groups: SwitcherGroup[]
  current: SwitcherGroup
  // адрес, который открывается при выборе группы
  hrefFor: (id: number) => string
  onCreate: () => void
  // меню открыли: можно обновить число сессий
  onOpen?: () => void
}

const itemClass =
  'grid min-h-10 w-full cursor-pointer grid-cols-[16px_minmax(0,1fr)_auto] items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-[14px] leading-5 font-semibold text-fg no-underline hover:text-fg hover:no-underline data-focus:bg-surface-hover max-md:min-h-11 [&>svg]:size-4'

// GroupSwitcher — название текущей группы в шапке и меню: группы с числом сессий и «Новая группа».
export function GroupSwitcher({ groups, current, hrefFor, onCreate, onOpen }: GroupSwitcherProps) {
  return (
    <Menu as="div" className="relative min-w-0">
      <MenuButton
        title={`Группа: ${current.name}`}
        onClick={onOpen}
        className="focus-ring flex h-9 max-w-[300px] cursor-pointer items-center gap-2 rounded-md border border-transparent pr-2 pl-2.5 text-[14px] leading-5 font-bold text-fg transition-colors select-none hover:bg-surface-hover data-open:border-border-strong data-open:bg-surface-2 max-md:max-w-full max-md:pr-1.5 max-md:pl-2"
      >
        <span className="min-w-0 truncate">{current.name}</span>
        <ChevronsUpDown className="size-4 shrink-0 text-fg-muted" aria-hidden />
      </MenuButton>
      <MenuItems
        anchor={{ to: 'bottom start', gap: 6 }}
        className="z-40 flex w-[300px] max-w-[calc(100vw-32px)] flex-col rounded-lg border border-border-strong bg-surface-2 p-1.5 shadow-overlay outline-none"
      >
        <div className="px-2.5 pt-2 pb-1.5 text-over font-bold uppercase text-fg-muted">Группы</div>
        <div className="flex max-h-[264px] flex-col gap-0.5 overflow-y-auto">
          {groups.map((g) => {
            const cur = g.id === current.id
            return (
              <MenuItem key={g.id}>
                <Link to={hrefFor(g.id)} title={g.name} aria-current={cur ? 'true' : undefined} className={itemClass}>
                  {cur ? <Check className="text-accent" aria-hidden /> : <span />}
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate">{g.name}</span>
                    <span className="truncate text-small font-medium text-fg-muted tabular-nums">
                      {g.sessionCount ? plural(g.sessionCount, ['сессия', 'сессии', 'сессий']) : 'нет сессий'}
                    </span>
                  </span>
                  <span />
                </Link>
              </MenuItem>
            )
          })}
        </div>
        <MenuSeparator className="-mx-1.5 my-1.5 h-px bg-border" />
        <MenuItem>
          <button type="button" className={cx(itemClass, 'border-0 bg-transparent font-sans')} onClick={onCreate}>
            <Plus className="text-fg-muted" aria-hidden />
            <span>Новая группа</span>
            <span />
          </button>
        </MenuItem>
      </MenuItems>
    </Menu>
  )
}
