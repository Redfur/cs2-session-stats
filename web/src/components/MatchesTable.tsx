import { GripVertical, Trash } from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent } from 'react'
import { Link } from 'react-router'
import type { Match } from '../api'
import { StatusBadge } from './ui/Badge'
import { Button } from './ui/Button'
import { cx } from './ui/cx'
import { Score } from './ui/Score'
import { rowLinkClass } from './ui/StatTable'

interface MatchesTableProps {
  // матчи в текущем порядке; номер матча — позиция в списке
  matches: Match[]
  // переставить матч с позиции from на позицию to
  onMove: (from: number, to: number) => void
  onDelete: (m: Match, number: number) => void
  // подсветить матч, который только что переставили
  movedId?: number | null
  labelledBy: string
}

interface DragState {
  id: number
  from: number
  startY: number
  // перетаскивание началось после сдвига указателя на несколько пикселей
  active: boolean
  // позиция линии вставки: перед строкой с этим индексом, matches.length — после последней
  ins: number | null
}

const DRAG_THRESHOLD = 4

// MatchesTable — список матчей сессии. Матч перетаскивается за ручку (мышь, тач — Pointer Events)
// на любую позицию, Esc отменяет перетаскивание. С клавиатуры на ручке ↑/↓ сдвигают матч на одну позицию.
export function MatchesTable({ matches, onMove, onDelete, movedId, labelledBy }: MatchesTableProps) {
  const rows = useRef(new Map<number, HTMLTableRowElement>())
  const grips = useRef(new Map<number, HTMLButtonElement>())
  const [drag, setDrag] = useState<DragState | null>(null)
  // ручка, на которой нужно сохранить фокус после перестановки с клавиатуры
  const [focusId, setFocusId] = useState<number | null>(null)

  useLayoutEffect(() => {
    if (focusId !== null) grips.current.get(focusId)?.focus()
  }, [focusId, matches])

  const dragging = drag?.active ?? false
  useEffect(() => {
    if (!dragging) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        setDrag(null)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [dragging])

  function insertionIndex(clientY: number): number {
    for (let k = 0; k < matches.length; k++) {
      const r = rows.current.get(matches[k].id)?.getBoundingClientRect()
      if (r && clientY < r.top + r.height / 2) return k
    }
    return matches.length
  }

  // целевая позиция после удаления матча с исходного места; null — порядок не меняется
  function target(d: DragState): number | null {
    if (d.ins === null) return null
    const to = d.ins > d.from ? d.ins - 1 : d.ins
    return to === d.from ? null : to
  }

  const handlers = (id: number, index: number) => ({
    onPointerDown: (e: PointerEvent<HTMLButtonElement>) => {
      if (e.button !== 0) return
      e.currentTarget.setPointerCapture(e.pointerId)
      setDrag({ id, from: index, startY: e.clientY, active: false, ins: null })
    },
    onPointerMove: (e: PointerEvent<HTMLButtonElement>) => {
      if (!drag || drag.id !== id) return
      if (!drag.active && Math.abs(e.clientY - drag.startY) < DRAG_THRESHOLD) return
      const ins = insertionIndex(e.clientY)
      if (!drag.active || ins !== drag.ins) setDrag({ ...drag, active: true, ins })
    },
    onPointerUp: () => {
      if (!drag || drag.id !== id) return
      const to = drag.active ? target(drag) : null
      setDrag(null)
      if (to !== null) onMove(drag.from, to)
    },
    onPointerCancel: () => setDrag(null),
    onKeyDown: (e: ReactKeyboardEvent<HTMLButtonElement>) => {
      const to = e.key === 'ArrowUp' ? index - 1 : e.key === 'ArrowDown' ? index + 1 : null
      if (to === null) return
      e.preventDefault()
      if (to < 0 || to >= matches.length) return
      setFocusId(id)
      onMove(index, to)
    },
  })

  const lineAt = drag?.active && target(drag) !== null ? drag.ins : null

  return (
    <div className="relative overflow-x-auto">
      <table className="w-full min-w-[860px] table-fixed border-collapse text-table tabular-nums" aria-labelledby={labelledBy}>
        <colgroup>
          <col style={{ width: 44 }} />
          <col style={{ width: 44 }} />
          <col style={{ width: 150 }} />
          <col style={{ width: 104 }} />
          <col />
          <col style={{ width: '32%' }} />
          <col style={{ width: 56 }} />
        </colgroup>
        <thead>
          <tr className="bg-surface-2 text-small font-semibold text-fg-muted">
            <th scope="col" className="h-10 border-b border-border px-2.5">
              <span className="sr-only">Порядок</span>
            </th>
            <th scope="col" className="border-b border-border px-2.5 text-right">
              #
            </th>
            <th scope="col" className="border-b border-border px-2.5 text-left">
              Карта
            </th>
            <th scope="col" className="border-b border-border px-2.5 text-right">
              Счёт A : B
            </th>
            <th scope="col" className="border-b border-border px-2.5 text-left">
              Статус
            </th>
            <th scope="col" className="border-b border-border px-2.5 text-left">
              Файл
            </th>
            <th scope="col" className="border-b border-border px-2.5">
              <span className="sr-only">Действия</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {matches.map((m, i) => {
            const n = i + 1
            const href = `/matches/${m.id}`
            const line =
              lineAt === i
                ? '[&>td]:shadow-[inset_0_2px_0_var(--color-accent)]'
                : lineAt === matches.length && i === matches.length - 1
                  ? '[&>td]:shadow-[inset_0_-2px_0_var(--color-accent)]'
                  : ''
            return (
              <tr
                key={m.id}
                ref={(el) => {
                  if (el) rows.current.set(m.id, el)
                  else rows.current.delete(m.id)
                }}
                className={cx(
                  'border-b border-border transition-colors last:border-b-0 [&>td]:px-2.5 [&>td]:py-2.5',
                  movedId === m.id && !dragging ? 'bg-surface-selected' : 'bg-surface hover:bg-surface-hover',
                  drag?.active && drag.id === m.id && 'opacity-40',
                  line,
                )}
              >
                <td>
                  <button
                    type="button"
                    ref={(el) => {
                      if (el) grips.current.set(m.id, el)
                      else grips.current.delete(m.id)
                    }}
                    aria-label={`Переместить матч ${n} — стрелки вверх и вниз`}
                    title="Перетащите, чтобы изменить порядок"
                    className="focus-ring grid h-8 w-7 cursor-grab touch-none place-items-center rounded-sm border-0 bg-transparent p-0 text-fg-faint transition-colors hover:bg-surface-2 hover:text-fg active:cursor-grabbing"
                    {...handlers(m.id, i)}
                  >
                    <GripVertical className="size-4" aria-hidden />
                  </button>
                </td>
                <td className="text-right">
                  <Link to={href} className="focus-ring font-bold">
                    {n}
                  </Link>
                </td>
                <td className="overflow-hidden text-ellipsis whitespace-nowrap">
                  {m.map ? (
                    <Link to={href} className={rowLinkClass}>
                      {m.map}
                    </Link>
                  ) : (
                    <span className="text-fg-faint">—</span>
                  )}
                </td>
                <td className="text-right">
                  {m.hasResult ? <Score a={m.scoreA} b={m.scoreB} /> : <span className="text-fg-faint">—</span>}
                </td>
                <td>
                  <StatusBadge match={m} />
                </td>
                <td className="overflow-hidden">
                  <span
                    className="block overflow-hidden font-mono text-small text-ellipsis whitespace-nowrap text-fg-muted"
                    title={m.originalName}
                  >
                    {m.originalName}
                  </span>
                </td>
                <td className="text-right">
                  <Button
                    variant="delete"
                    size="sm"
                    icon
                    aria-label={`Удалить матч ${n}`}
                    title="Удалить матч"
                    onClick={() => onDelete(m, n)}
                  >
                    <Trash aria-hidden />
                  </Button>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
