import { Inbox, RotateCcw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { api, type PlayerRow } from '../api'
import { PeriodSide } from '../components/PeriodFilter'
import { PLAYER_TOTAL_COLUMNS, PLAYER_TOTAL_MIN_WIDTH, RATING_DESC } from '../components/playerColumns'
import { Alert } from '../components/ui/Alert'
import { Button, ButtonLink } from '../components/ui/Button'
import { Card, CardBody, CardHeader } from '../components/ui/Card'
import { Checkbox } from '../components/ui/Checkbox'
import { EmptyState } from '../components/ui/EmptyState'
import { Input, Label } from '../components/ui/Field'
import { Page } from '../components/ui/Layout'
import { PageHeader } from '../components/ui/PageHeader'
import { SortHint, StatTable } from '../components/ui/StatTable'
import { updateParams } from '../filters'
import { plural } from '../format'
import { periodText, usePeriod, useSessionList } from '../period'
import { sortRows, useSort } from '../sort'

const DEFAULT_MIN_MATCHES = '3'

export function PlayersPage() {
  const [params, setParams] = useSearchParams()
  const [players, setPlayers] = useState<PlayerRow[] | null>(null)
  const [options, setOptions] = useState<PlayerRow[]>([])
  const [error, setError] = useState('')
  const sessions = useSessionList()
  const period = usePeriod(params, setParams)
  const { sort, toggle } = useSort('sort', PLAYER_TOTAL_COLUMNS, RATING_DESC)

  const minMatches = params.get('minMatches') ?? DEFAULT_MIN_MATCHES
  const selected = new Set(params.getAll('player'))
  const update = (change: (p: URLSearchParams) => void) => updateParams(params, setParams, change)

  // в запрос идут только фильтры: порог по умолчанию передаётся явно, сортировка остаётся на клиенте
  const query = useMemo(() => {
    const q = new URLSearchParams(params)
    q.delete('sort')
    q.set('minMatches', minMatches)
    return q.toString()
  }, [params, minMatches])

  useEffect(() => {
    let stale = false
    api.listPlayers(query).then(
      (r) => {
        if (stale) return
        setPlayers(r.players)
        setError('')
      },
      (e: Error) => !stale && setError(e.message),
    )
    return () => {
      stale = true
    }
  }, [query])

  // варианты мультивыбора — все игроки без фильтров
  useEffect(() => {
    api.listPlayers('').then(
      (r) => setOptions([...r.players].sort((a, b) => a.name.localeCompare(b.name, 'ru', { sensitivity: 'base' }))),
      () => setOptions([]),
    )
  }, [])

  function togglePlayer(id: string) {
    update((p) => {
      const ids = p.getAll('player').filter((x) => x !== id)
      if (!selected.has(id)) ids.push(id)
      p.delete('player')
      for (const x of ids) p.append('player', x)
    })
  }

  function setMinMatches(value: string) {
    const q = new URLSearchParams(params)
    if (value === DEFAULT_MIN_MATCHES) q.delete('minMatches')
    else q.set('minMatches', value)
    setParams(q, { replace: true }) // ввод числа не должен плодить записи истории
  }

  const reset = () => setParams(new URLSearchParams())

  const totalMatches = sessions.reduce((n, s) => n + s.matchCount, 0)
  const sessionsWithMatches = sessions.filter((s) => s.matchCount > 0).length
  const rows = players ? sortRows(players, PLAYER_TOTAL_COLUMNS, sort) : []
  const min = Number(minMatches) || 1

  return (
    <Page>
      <PageHeader
        title="Игроки"
        meta={
          options.length > 0 && (
            <span className="tabular-nums">
              {plural(options.length, ['игрок', 'игрока', 'игроков'])} · {plural(totalMatches, ['матч', 'матча', 'матчей'])} в{' '}
              {plural(sessionsWithMatches, ['сессии', 'сессиях', 'сессиях'])}
            </span>
          )
        }
      />

      <div className="flex flex-wrap items-start gap-6">
        <aside className="flex flex-[1_1_280px] flex-col gap-4 md:max-w-[300px]" aria-label="Фильтры">
          <Card as="div">
            <div className="flex min-h-[52px] items-center justify-between gap-3 border-b border-border px-4 py-2.5 md:px-5">
              <h2 className="m-0 text-sub">Фильтры</h2>
              <Button variant="ghost" size="sm" leading={<RotateCcw aria-hidden />} onClick={reset}>
                Сбросить
              </Button>
            </div>
            <CardBody className="border-b border-border">
              <PeriodSide period={period} sessions={sessions} />
            </CardBody>
            <CardBody className="flex flex-col gap-2 border-b border-border">
              <div className="flex items-baseline justify-between">
                <Label>Игроки</Label>
                <span className="text-small text-fg-muted">матчей</span>
              </div>
              <div className="flex flex-col">
                {options.length === 0 && <span className="text-small text-fg-muted">Игроков нет.</span>}
                {options.map((p) => (
                  <Checkbox
                    key={p.steamId}
                    className="min-h-[30px]"
                    checked={selected.has(p.steamId)}
                    onChange={() => togglePlayer(p.steamId)}
                    count={p.matches ?? 0}
                  >
                    <span className="overflow-hidden text-ellipsis whitespace-nowrap">{p.name}</span>
                  </Checkbox>
                ))}
              </div>
              <div className="my-1.5 h-px bg-border" />
              <Checkbox
                checked={params.get('together') === '1'}
                disabled={selected.size < 2}
                onChange={(on) => update((p) => (on ? p.set('together', '1') : p.delete('together')))}
              >
                только совместные матчи
              </Checkbox>
              <span className="pl-7 text-small text-fg-muted">
                {selected.size < 2 ? 'Отметьте хотя бы двух игроков' : 'Только матчи, где все отмеченные играли вместе'}
              </span>
            </CardBody>
            <CardBody className="flex items-center justify-between gap-3">
              <label htmlFor="pf-min" className="text-[13px] font-medium whitespace-nowrap">
                Минимум матчей
              </label>
              <Input
                id="pf-min"
                type="number"
                min={1}
                className="w-[84px] text-right"
                value={minMatches}
                onChange={(e) => setMinMatches(e.target.value)}
              />
            </CardBody>
          </Card>
          <p className="m-0 px-1 text-small text-fg-muted">Фильтры и сортировка хранятся в адресе страницы — ссылкой можно поделиться.</p>
        </aside>

        <div className="flex min-w-0 flex-[999_1_560px] flex-col gap-4">
          {error ? (
            <Card as="div">
              <CardBody>
                <Alert
                  tone="error"
                  title="Фильтр не применён."
                  action={
                    <Button size="sm" onClick={reset}>
                      Сбросить фильтры
                    </Button>
                  }
                >
                  В ссылке некорректный фильтр, поэтому таблица не построена.
                  <span className="mt-1 block text-small text-fg-muted">Ответ сервера: {error}</span>
                </Alert>
              </CardBody>
            </Card>
          ) : (
            players && (
              <Card aria-labelledby="players-h">
                <CardHeader
                  title="Сводная таблица"
                  titleId="players-h"
                  note={`${plural(rows.length, ['игрок', 'игрока', 'игроков'])} · ${periodText(period)} · от ${min} ${min === 1 ? 'матча' : 'матчей'}`}
                  aside={<SortHint columns={PLAYER_TOTAL_COLUMNS} sort={sort} />}
                />
                {rows.length > 0 ? (
                  <StatTable
                    columns={PLAYER_TOTAL_COLUMNS}
                    rows={rows}
                    rowKey={(p) => p.steamId}
                    sort={sort}
                    onSort={toggle}
                    highlight={(p) => selected.has(p.steamId)}
                    labelledBy="players-h"
                    minWidth={PLAYER_TOTAL_MIN_WIDTH}
                  />
                ) : options.length === 0 ? (
                  <EmptyState
                    icon={<Inbox />}
                    title="Данных пока нет"
                    text="Таблица появится, когда в сессиях будут обработанные матчи."
                    actions={
                      <ButtonLink to="/" variant="secondary" size="sm">
                        К сессиям
                      </ButtonLink>
                    }
                  />
                ) : (
                  <EmptyState
                    icon={<Inbox />}
                    title="Под фильтры никто не подошёл"
                    text="Уменьшите «Минимум матчей» или расширьте период."
                    actions={
                      <Button variant="ghost" size="sm" leading={<RotateCcw aria-hidden />} onClick={reset}>
                        Сбросить фильтры
                      </Button>
                    }
                  />
                )}
              </Card>
            )
          )}
        </div>
      </div>
    </Page>
  )
}
