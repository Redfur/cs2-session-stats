import { Popover, PopoverButton, PopoverPanel } from '@headlessui/react'
import { ChevronDown } from 'lucide-react'
import type { SessionSummary } from '../api'
import { formatDateShort } from '../format'
import type { Period, PeriodMode } from '../period'
import { buttonClass } from './ui/buttonClass'
import { Checkbox } from './ui/Checkbox'
import { Field, Input, Label } from './ui/Field'
import { Segment } from './ui/Segment'

const MODES: { value: PeriodMode; label: string }[] = [
  { value: 'dates', label: 'Даты' },
  { value: 'sessions', label: 'Сессии' },
]

// Список сессий с чекбоксами; сессия без названия показывается датой.
export function SessionChecklist({ period, sessions }: { period: Period; sessions: SessionSummary[] }) {
  if (sessions.length === 0) return <span className="text-small text-fg-muted">Сессий нет.</span>
  return (
    <div className="flex flex-col gap-1.5">
      {sessions.map((s) => (
        <Checkbox
          key={s.id}
          checked={period.selected.has(String(s.id))}
          onChange={() => period.toggleSession(String(s.id))}
          count={s.matchCount}
        >
          <span className="overflow-hidden text-ellipsis whitespace-nowrap">{s.title || formatDateShort(s.date)}</span>
          <span className="text-small text-fg-muted">{s.title ? formatDateShort(s.date) : 'без названия'}</span>
        </Checkbox>
      ))}
    </div>
  )
}

function DateFields({ period, idPrefix, className }: { period: Period; idPrefix: string; className?: string }) {
  return (
    <>
      <Field label="С" htmlFor={`${idPrefix}-from`} className={className}>
        <Input id={`${idPrefix}-from`} type="date" value={period.from} onChange={(e) => period.setDate('from', e.target.value)} />
      </Field>
      <Field label="По" htmlFor={`${idPrefix}-to`} className={className}>
        <Input id={`${idPrefix}-to`} type="date" value={period.to} onChange={(e) => period.setDate('to', e.target.value)} />
      </Field>
    </>
  )
}

// PeriodSide — фильтр периода в боковой панели: сегмент режима, под ним даты или список сессий.
export function PeriodSide({ period, sessions }: { period: Period; sessions: SessionSummary[] }) {
  return (
    <div className="flex flex-col gap-3">
      <Label>Период</Label>
      <Segment label="Режим периода" value={period.mode} onChange={period.setMode} options={MODES} />
      {period.mode === 'dates' ? (
        <>
          <div className="grid grid-cols-2 gap-2.5">
            <DateFields period={period} idPrefix="pf" />
          </div>
          <span className="text-small text-fg-muted">Пусто — за всё время</span>
        </>
      ) : (
        <SessionChecklist period={period} sessions={sessions} />
      )}
    </div>
  )
}

// PeriodBar — компактный фильтр периода над таблицами; сессии выбираются в поповере.
export function PeriodBar({ period, sessions }: { period: Period; sessions: SessionSummary[] }) {
  return (
    <div className="flex flex-wrap items-end gap-3" role="group" aria-label="Период">
      <div className="flex flex-col gap-1.5">
        <Label>Период</Label>
        <Segment label="Режим периода" value={period.mode} onChange={period.setMode} options={MODES} />
      </div>
      {period.mode === 'dates' ? (
        <div className="grid grid-cols-2 gap-3 sm:flex">
          <DateFields period={period} idPrefix="pp" className="sm:w-[150px]" />
        </div>
      ) : (
        <Popover className="relative">
          <PopoverButton className={buttonClass({ variant: 'secondary' })}>
            Выбрать сессии · {period.selected.size}
            <ChevronDown className="size-4" aria-hidden />
          </PopoverButton>
          <PopoverPanel
            anchor={{ to: 'bottom end', gap: 6 }}
            className="z-40 max-h-[360px] w-[300px] max-w-[calc(100vw-32px)] overflow-y-auto rounded-lg border border-border-strong bg-surface p-4 shadow-overlay"
          >
            <SessionChecklist period={period} sessions={sessions} />
          </PopoverPanel>
        </Popover>
      )}
    </div>
  )
}
