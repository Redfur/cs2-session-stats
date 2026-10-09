// Сортировка таблиц статистики с хранением в адресе страницы.
// Значение параметра: «колонка» (направление по умолчанию для колонки) или «колонка.asc» / «колонка.desc».
// Сортировка по умолчанию в адрес не пишется, неизвестная колонка игнорируется.

import { useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router'

export type SortDir = 'asc' | 'desc'

export interface SortState {
  key: string
  dir: SortDir
}

export interface SortableColumn<T> {
  key: string
  // числовая колонка при первом клике сортируется по убыванию, текстовая — по алфавиту
  numeric?: boolean
  value: (row: T) => number | string
}

function columnDir(col: { numeric?: boolean }): SortDir {
  return col.numeric ? 'desc' : 'asc'
}

export function parseSort<T>(raw: string | null, columns: SortableColumn<T>[], fallback: SortState): SortState {
  if (!raw) return fallback
  const [key, dir, ...rest] = raw.split('.')
  const col = columns.find((c) => c.key === key)
  if (!col || rest.length > 0) return fallback
  if (dir === undefined) return { key, dir: columnDir(col) }
  if (dir === 'asc' || dir === 'desc') return { key, dir }
  return fallback
}

function formatSort<T>(s: SortState, columns: SortableColumn<T>[]): string {
  const col = columns.find((c) => c.key === s.key)
  return col && columnDir(col) === s.dir ? s.key : `${s.key}.${s.dir}`
}

const collator = new Intl.Collator('ru', { sensitivity: 'base', numeric: true })

// Стабильная сортировка: при равенстве значений строки остаются в исходном порядке.
export function sortRows<T>(rows: T[], columns: SortableColumn<T>[], s: SortState): T[] {
  const col = columns.find((c) => c.key === s.key)
  if (!col) return rows
  const sign = s.dir === 'asc' ? 1 : -1
  return rows
    .map((row, i) => ({ row, i, v: col.value(row) }))
    .sort((a, b) => {
      const c =
        typeof a.v === 'number' && typeof b.v === 'number' ? a.v - b.v : collator.compare(String(a.v), String(b.v))
      return c !== 0 ? sign * c : a.i - b.i
    })
    .map((x) => x.row)
}

// Следующее состояние по клику на заголовок: повторный клик меняет направление.
export function nextSort<T>(current: SortState, columns: SortableColumn<T>[], key: string): SortState {
  if (current.key === key) return { key, dir: current.dir === 'asc' ? 'desc' : 'asc' }
  const col = columns.find((c) => c.key === key)
  return { key, dir: col ? columnDir(col) : 'desc' }
}

// useSort читает сортировку таблицы из параметра адреса param и возвращает её с функцией клика по колонке.
// Клики по сортировке заменяют запись истории, а не добавляют новую.
export function useSort<T>(param: string, columns: SortableColumn<T>[], fallback: SortState) {
  const [params, setParams] = useSearchParams()
  const raw = params.get(param)
  const state = useMemo(() => parseSort(raw, columns, fallback), [raw, columns, fallback])

  const toggle = useCallback(
    (key: string) => {
      const next = nextSort(state, columns, key)
      setParams(
        (prev) => {
          const p = new URLSearchParams(prev)
          if (next.key === fallback.key && next.dir === fallback.dir) p.delete(param)
          else p.set(param, formatSort(next, columns))
          return p
        },
        { replace: true },
      )
    },
    [state, columns, fallback, param, setParams],
  )

  return { sort: state, toggle }
}
