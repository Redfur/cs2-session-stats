import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'
import type { ReactNode } from 'react'
import type { Metric } from '../../metrics'
import type { SortableColumn, SortState } from '../../sort'
import { cx } from './cx'
import { MetricValue } from './MetricValue'

export interface Column<T> extends SortableColumn<T> {
  label: ReactNode
  // подсказка у заголовка: полное название показателя
  title?: string
  // текстовая подпись колонки для подсказки о сортировке, если label — не строка
  name?: string
  render?: (row: T) => ReactNode
  // окрашивание по шкале показателей
  metric?: Metric
  strong?: boolean
  // второстепенная колонка: на узком экране приглушена
  secondary?: boolean
  // ширина колонки в px; у первой колонки обычно не задаётся — она забирает остаток
  width?: number
  // выравнивание: по умолчанию числовые — вправо, текстовые — влево
  align?: 'left' | 'right'
}

interface StatTableProps<T> {
  columns: Column<T>[]
  // строки уже отсортированы
  rows: T[]
  rowKey: (row: T) => string | number
  sort?: SortState
  onSort?: (key: string) => void
  // выделенная строка: текущий игрок или игроки из фильтра
  highlight?: (row: T) => boolean
  labelledBy?: string
  label?: string
  // минимальная ширина таблицы: при меньшей ширине карточки таблица прокручивается внутри неё
  minWidth: number
}

function alignOf<T>(c: Column<T>) {
  return c.align ?? (c.numeric ? 'right' : 'left')
}

const STICKY = 'sticky left-0 z-[1] border-r border-border bg-inherit'

// StatTable — таблица статистики: сортировка по заголовкам, закреплённая первая колонка,
// прокрутка по горизонтали внутри своего блока.
export function StatTable<T>({ columns, rows, rowKey, sort, onSort, highlight, labelledBy, label, minWidth }: StatTableProps<T>) {
  return (
    <div className="relative overflow-x-auto [-webkit-overflow-scrolling:touch]">
      <table
        className="w-full table-fixed border-collapse text-table tabular-nums"
        style={{ minWidth }}
        aria-labelledby={labelledBy}
        aria-label={label}
      >
        <colgroup>
          {columns.map((c) => (
            <col key={c.key} style={c.width ? { width: c.width } : undefined} />
          ))}
        </colgroup>
        <thead>
          <tr className="bg-surface-2">
            {columns.map((c, i) => {
              const active = sort?.key === c.key
              const left = alignOf(c) === 'left'
              const Icon = active ? (sort?.dir === 'asc' ? ArrowUp : ArrowDown) : ChevronsUpDown
              const icon = (
                <Icon
                  className={cx(
                    'size-3 flex-none transition-opacity',
                    active ? 'text-accent opacity-100' : 'opacity-0 group-hover/sort:opacity-55',
                  )}
                  aria-hidden
                />
              )
              return (
                <th
                  key={c.key}
                  scope="col"
                  title={onSort ? undefined : c.title}
                  aria-sort={active ? (sort?.dir === 'asc' ? 'ascending' : 'descending') : undefined}
                  className={cx(
                    'h-10 border-b border-border text-small font-semibold whitespace-nowrap',
                    active ? 'text-fg' : 'text-fg-muted',
                    left ? 'text-left' : 'text-right',
                    i === 0 && STICKY,
                    !onSort && 'px-2.5',
                  )}
                >
                  {onSort ? (
                    <button
                      type="button"
                      title={c.title}
                      onClick={() => onSort(c.key)}
                      className={cx(
                        'group/sort focus-ring flex h-10 w-full cursor-pointer items-center gap-1 border-0 bg-transparent px-2.5 font-[inherit] text-inherit hover:text-fg',
                        left ? 'justify-start' : 'justify-end',
                      )}
                    >
                      {!left && icon}
                      {c.label}
                      {left && icon}
                    </button>
                  ) : (
                    c.label
                  )}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const hl = highlight?.(row) ?? false
            return (
              <tr
                key={rowKey(row)}
                data-highlight={hl || undefined}
                className={cx(
                  'group/row border-b border-border transition-colors last:border-b-0',
                  hl ? 'bg-surface-selected' : 'bg-surface hover:bg-surface-hover',
                )}
              >
                {columns.map((c, i) => {
                  const v = c.value(row)
                  const content = c.render
                    ? c.render(row)
                    : c.metric && typeof v === 'number'
                      ? <MetricValue metric={c.metric} value={v} strong={c.strong} />
                      : v
                  const Cell = i === 0 ? 'th' : 'td'
                  return (
                    <Cell
                      key={c.key}
                      scope={i === 0 ? 'row' : undefined}
                      className={cx(
                        'h-10 overflow-hidden px-2.5 font-medium text-ellipsis whitespace-nowrap',
                        alignOf(c) === 'left' ? 'text-left' : 'text-right',
                        c.secondary && 'max-md:text-fg-muted',
                        i === 0 && STICKY,
                      )}
                    >
                      {content}
                    </Cell>
                  )
                })}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// SortHint — подсказка в шапке карточки: текущая сортировка.
export function SortHint<T>({ columns, sort, prefix = 'Сортировка: ' }: { columns: Column<T>[]; sort: SortState; prefix?: string }) {
  const col = columns.find((c) => c.key === sort.key)
  const name = col?.name ?? (typeof col?.label === 'string' ? col.label : sort.key)
  return (
    <span className="text-small text-fg-muted">
      {prefix}
      {name} {sort.dir === 'asc' ? '↑' : '↓'}
    </span>
  )
}

// Ссылка на игрока в первой колонке; в выделенной строке — акцентная.
export const rowLinkClass =
  'focus-ring font-semibold text-fg hover:text-accent-hover group-data-[highlight]/row:text-accent'
