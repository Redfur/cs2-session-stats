import { CalendarX } from 'lucide-react'
import type { ReactNode } from 'react'
import {
  api,
  sessionTitle,
  type ClutchRow,
  type ExtHeader,
  type ExtMetric,
  type GrenadeMap,
  type PlayerFight,
  type PlayerUtility,
  type PlayerWeapons,
  type SessionSummary,
  type SurvivalRow,
} from '../api'
import { plural } from '../format'
import { sortRows, useSort, type SortState } from '../sort'
import { coverageNote, fmt1, fmtInt, fmtPct, fmtSigned, missingTip, sharePct, useExtLoad, type Load } from '../ext'
import { CoverageBadge, ExtAlerts, ExtError, ExtLoading, ExtNotComputed, MetricNum } from './ExtStats'
import { ButtonLink } from './ui/Button'
import { Card, CardFooter, CardHeader } from './ui/Card'
import { EmptyState } from './ui/EmptyState'
import { Checking, CoverageChip, NotComputed, ShareBar } from './ui/ExtValue'
import { Kpi, KpiGrid } from './ui/KpiGrid'
import { SideChip } from './ui/SideChip'
import { StatTable, type Column } from './ui/StatTable'
import { weaponColumns } from './weaponColumns'

interface TabProps {
  steamId: string
  // период профиля в виде query string
  query: string
  sessions: SessionSummary[]
}

const Muted = ({ children }: { children: ReactNode }) => <span className="text-small text-fg-muted">{children}</span>

// ExtTab — общая обвязка вкладки: одно состояние на всю вкладку, пересчёт — Alert сверху.
function ExtTab<T extends ExtHeader>({
  load,
  sessions,
  children,
}: {
  load: Load<T>
  sessions: SessionSummary[]
  children: (data: T) => ReactNode
}) {
  const { data, error, reload } = load
  const session = (id: number) => {
    const s = sessions.find((x) => x.id === id)
    return s ? { id, title: sessionTitle(s) } : undefined
  }
  if (error && !data)
    return (
      <ExtError
        title="Не удалось загрузить статистику раздела."
        text="Сервер не ответил. Плитки выше и «Обзор» это не затрагивает."
        onRetry={reload}
      />
    )
  if (!data) return <ExtLoading label="Загружаем статистику раздела" />
  const alerts = (
    <ExtAlerts
      header={data}
      sessionLink={(id) => (
        <ButtonLink to={`/sessions/${id}`} size="sm" variant="secondary">
          К сессии
        </ButtonLink>
      )}
    />
  )
  if (data.status === 'no_matches')
    return (
      <Card as="div">
        <EmptyState icon={<CalendarX />} as="h2" title="Нет игр за выбранный период" text="Измените даты или выберите другие сессии." />
      </Card>
    )
  if (data.status === 'unavailable')
    return (
      <>
        {alerts}
        <ExtNotComputed
          text="Все матчи за выбранный период обработаны старой версией. Пересчитайте их на страницах сессий — плитки выше и «Обзор» уже доступны."
          session={data.uncoveredMatches[0] && session(data.uncoveredMatches[0].sessionId)}
        />
      </>
    )
  return (
    <div className="flex flex-col gap-6">
      {alerts}
      {children(data)}
    </div>
  )
}

function Header({ id, title, note, metric, aside }: { id: string; title: string; note?: ReactNode; metric?: ExtMetric; aside?: ReactNode }) {
  return (
    <CardHeader
      title={title}
      titleId={id}
      note={
        (note || metric) && (
          <span className="inline-flex flex-wrap items-center gap-3">
            {note}
            {metric && <CoverageBadge metric={metric} />}
          </span>
        )
      }
      aside={aside}
    />
  )
}

// Доля: число справа и нейтральная полоса; без знаменателя — прочерк с причиной.
function Share({ value, reason }: { value: number | null; reason: string }) {
  if (value == null) return <NotComputed reason={reason} />
  return (
    <span className="inline-flex items-center justify-end">
      {fmtPct(value)}
      <ShareBar share={value / 100} />
    </span>
  )
}

// ── «Бой» ──

const CLUTCH_COLUMNS: Column<ClutchRow & { name: string }>[] = [
  { key: 'name', label: 'Ситуация', value: (r) => r.name, render: (r) => <span className="font-bold">{r.name}</span> },
  { key: 'attempts', label: 'Попыток', numeric: true, width: 92, value: (r) => r.attempts },
  { key: 'wins', label: 'Победы', numeric: true, width: 84, value: (r) => r.wins },
  { key: 'losses', label: 'Поражения', numeric: true, secondary: true, width: 104, value: (r) => r.losses },
  {
    key: 'draws',
    label: 'Ничьи',
    title: 'Раунд закончился без победителя',
    numeric: true,
    secondary: true,
    width: 76,
    value: (r) => r.draws,
  },
  {
    key: 'unknown',
    label: 'Исход неизвестен',
    title: 'Демка оборвалась в этом раунде, исход клатча не определён',
    numeric: true,
    secondary: true,
    width: 140,
    value: (r) => r.unknown,
  },
  {
    key: 'rate',
    label: 'Доля побед',
    title: 'Победы / (победы + поражения). Ничьи и неизвестный исход не учитываются',
    numeric: true,
    width: 140,
    value: (r) => r.winRate ?? -1,
    render: (r) => (
      <Share
        value={r.winRate == null ? null : r.winRate * 100}
        reason={r.attempts ? 'Нет выигранных или проигранных попыток — долю не считаем' : 'Попыток не было — долю не считаем'}
      />
    ),
  },
]

const SIDE_NAME = { T: 'T', CT: 'CT', total: 'Всего' } as const

const SURVIVAL_COLUMNS: Column<SurvivalRow>[] = [
  {
    key: 'side',
    label: 'Сторона',
    value: (r) => r.side,
    render: (r) => (r.side === 'total' ? <span className="font-bold">Всего</span> : <SideChip side={r.side} />),
  },
  { key: 'rounds', label: 'Раундов', numeric: true, secondary: true, width: 84, value: (r) => r.rounds },
  { key: 'survived', label: 'Выжил', numeric: true, width: 72, value: (r) => r.survived },
  {
    key: 'share',
    label: 'Доля',
    numeric: true,
    width: 120,
    value: (r) => r.share ?? -1,
    render: (r) => <Share value={r.share == null ? null : r.share * 100} reason="Раундов не было — долю не считаем" />,
  },
  {
    key: 'alive',
    label: 'Время жизни',
    title: 'Среднее время от начала раунда до смерти или до конца раунда',
    numeric: true,
    width: 112,
    value: (r) => r.avgAliveSec ?? -1,
    render: (r) => (r.avgAliveSec == null ? <NotComputed reason="Раундов не было" /> : `${r.avgAliveSec} с`),
  },
]

export function FightTab({ steamId, query, sessions }: TabProps) {
  const load = useExtLoad<PlayerFight>(() => api.getPlayerFight(steamId, query), `${steamId}|${query}`)
  return (
    <ExtTab load={load} sessions={sessions}>
      {(d: PlayerFight) => {
        const t = d.trades
        const dmg = d.damage
        const c = d.clutches.sum
        const a = d.assists
        const tradedShare = sharePct(t.tradedDeaths, t.deaths)
        const tradeShare = sharePct(t.tradeKills, t.kills)
        const clutchRows = [
          ...d.clutches.rows.map((r) => ({ ...r, name: `1v${r.vs}` })),
          { ...c, name: 'Всего' },
        ]
        return (
          <>
            <div className="grid grid-cols-1 items-start gap-6 md:grid-cols-2">
              <Card aria-labelledby="tr-h">
                <Header id="tr-h" title="Размены" note="в течение 5 с" metric={t} />
                <KpiGrid cols={2}>
                  <Kpi
                    label="Ваши смерти разменяли"
                    value={t.tradedDeaths}
                    of={`/ ${t.deaths}`}
                    sub={tradedShare == null ? 'смертей не было' : `${tradedShare}% смертей`}
                  />
                  <Kpi
                    label="Убийств в размен"
                    value={t.tradeKills}
                    sub={tradeShare == null ? 'убийств не было' : `${tradeShare}% ваших убийств`}
                  />
                </KpiGrid>
                <CardFooter className="text-small text-fg-muted">
                  Размен — союзник убил вашего убийцу не позже 5 с после вашей смерти. Убийство в размен — вы так же
                  отомстили за союзника.
                </CardFooter>
              </Card>

              <Card aria-labelledby="dm-h">
                <Header id="dm-h" title="Урон" note="по противникам, за раунд" metric={dmg} />
                <KpiGrid cols={3}>
                  <Kpi label="Наносит" value={<MetricNum metric={{ ...dmg, value: dmg.dealtPerRound }} format={fmt1} />} sub={dmgRounds(dmg)} />
                  <Kpi label="Получает" value={<MetricNum metric={{ ...dmg, value: dmg.takenPerRound }} format={fmt1} />} sub={dmgRounds(dmg)} />
                  <Kpi label="Разница" value={<MetricNum metric={{ ...dmg, value: dmg.diffPerRound }} format={fmtSigned} />} sub="нанесено − получено" />
                </KpiGrid>
                <CardFooter className="text-small text-fg-muted">
                  Разница = нанесённый урон − полученный, в среднем за раунд. Урон по своим не учитывается.
                </CardFooter>
              </Card>
            </div>

            <Card aria-labelledby="cl-h">
              <Header
                id="cl-h"
                title="Клатчи"
                note="остался один против нескольких"
                metric={d.clutches}
                aside={
                  <Muted>
                    {c.wins + c.losses > 0
                      ? `Выиграно ${c.wins} из ${c.wins + c.losses} · ${Math.round((c.winRate ?? 0) * 100)}%`
                      : c.attempts > 0
                        ? 'решённых попыток нет'
                        : 'попыток не было'}
                  </Muted>
                }
              />
              <StatTable columns={CLUTCH_COLUMNS} rows={clutchRows} rowKey={(r) => r.name} labelledBy="cl-h" minWidth={760} />
              <CardFooter className="text-small text-fg-muted">
                Доля побед считается только по выигранным и проигранным попыткам. Нет таких попыток — прочерк, а не 0%.
              </CardFooter>
            </Card>

            <div className="grid grid-cols-1 items-start gap-6 md:grid-cols-2">
              <Card aria-labelledby="sv-h">
                <Header id="sv-h" title="Выживание" note="дожил до конца раунда" metric={d.survival} />
                <StatTable columns={SURVIVAL_COLUMNS} rows={d.survival.rows} rowKey={(r) => SIDE_NAME[r.side]} labelledBy="sv-h" minWidth={480} />
              </Card>

              <Card aria-labelledby="as-h">
                <Header id="as-h" title="Ассисты" note={`всего ${a.total}`} />
                <KpiGrid cols={3}>
                  <Kpi label="Флеш-ассисты" value={a.flash} sub="ослепил — союзник добил" />
                  <Kpi label="Обычные" value={a.damage} sub="урон — союзник добил" />
                  <Kpi label="Тип неизвестен" value={a.unknown} sub="старые матчи" />
                </KpiGrid>
                <CardFooter className="text-small text-fg-muted">
                  Флеш-ассист — противника убили, пока он был ослеплён вашей флешкой. В матчах, обработанных до
                  обновления, тип ассиста не записан.
                </CardFooter>
              </Card>
            </div>
          </>
        )
      }}
    </ExtTab>
  )
}

const dmgRounds = (d: { rounds: number; covered: number }) =>
  d.covered === 0 ? 'нет данных' : `по ${plural(d.rounds, ['раунду', 'раундам', 'раундам'])}`

// ── «Оружие» ──

const WEAPONS_DEFAULT: SortState = { key: 'kills', dir: 'desc' }
const WEAPON_COLUMNS_FULL = weaponColumns(true)
const WEAPON_COLUMNS_NO_DAMAGE = weaponColumns(false)

export function WeaponsTab({ steamId, query, sessions }: TabProps) {
  const load = useExtLoad<PlayerWeapons>(() => api.getPlayerWeapons(steamId, query), `${steamId}|${query}`)
  const columns = load.data && load.data.weapons.damage.covered === 0 ? WEAPON_COLUMNS_NO_DAMAGE : WEAPON_COLUMNS_FULL
  const sw = useSort('sw', columns, WEAPONS_DEFAULT)
  return (
    <ExtTab load={load} sessions={sessions}>
      {(d: PlayerWeapons) => {
        const k = d.killDetails
        const share = (n: number) => (k.kills ? `${Math.round((100 * n) / k.kills)}% убийств` : 'убийств не было')
        return (
          <>
            <Card aria-labelledby="wp-h">
              <Header
                id="wp-h"
                title="Оружие"
                note={plural(d.weapons.rows.length, ['вид', 'вида', 'видов'])}
                metric={d.weapons}
                aside={<Muted>HS% убийств и HS% попаданий — разные показатели</Muted>}
              />
              {d.weapons.rows.length > 0 ? (
                <StatTable
                  columns={columns}
                  rows={sortRows(d.weapons.rows, columns, sw.sort)}
                  rowKey={(r) => r.weapon}
                  sort={sw.sort}
                  onSort={sw.toggle}
                  labelledBy="wp-h"
                  minWidth={780}
                />
              ) : (
                <p className="m-0 px-4 py-5 text-[13px] text-fg-muted md:px-5">Убийств и урона из оружия за период нет.</p>
              )}
              <CardFooter className="text-small text-fg-muted">
                Выстрелы и попадания — счётчики; процент точности из демки надёжно не посчитать, поэтому его нет.
                {d.grenadeKills > 0 && ` Ещё ${plural(d.grenadeKills, ['убийство', 'убийства', 'убийств'])} гранатами — во вкладке «Гранаты и бомба».`}
              </CardFooter>
            </Card>

            <Card aria-labelledby="kd-h">
              <Header id="kd-h" title="Детали убийств" note={`из ${plural(k.kills, ['убийства', 'убийств', 'убийств'])}`} metric={k} />
              <KpiGrid cols={5}>
                <Kpi label="Через дым" value={k.smoke} sub={share(k.smoke)} />
                <Kpi label="Прострелы" value={k.wallbang} sub={share(k.wallbang)} />
                <Kpi label="Без прицела" value={k.noScope} sub={share(k.noScope)} />
                <Kpi label="Сам ослеплён" value={k.blind} sub={share(k.blind)} />
                <Kpi
                  label="Средняя дистанция"
                  value={k.avgDistance == null ? <NotComputed reason="Нет убийств с известной дистанцией" /> : fmt1(k.avgDistance)}
                  of={k.avgDistance == null ? undefined : 'м'}
                  sub="на одно убийство"
                />
              </KpiGrid>
            </Card>
          </>
        )
      }}
    </ExtTab>
  )
}

// ── «Гранаты и бомба» ──

function sameCoverage(list: ExtMetric[]) {
  return list.every((m) => m.covered === list[0].covered)
}

function GrenadeKpi({ label, metric, sub, uniform }: { label: string; metric: ExtMetric; sub: string; uniform: boolean }) {
  const note = uniform ? null : coverageNote(metric)
  return (
    <Kpi
      label={label}
      value={<MetricNum metric={metric} format={fmt1} />}
      sub={sub}
      coverage={note && <CoverageChip title={missingTip(metric.missing)}>{note}</CoverageChip>}
    />
  )
}

const mapCell = (m: ExtMetric, format: (v: number) => ReactNode = fmt1) => <MetricNum metric={m} format={format} />

const GRENADE_MAP_COLUMNS: Column<GrenadeMap>[] = [
  { key: 'map', label: 'Карта', value: (r) => r.map, render: (r) => <span className="font-semibold">{r.map || '—'}</span> },
  { key: 'he', label: 'Урон HE', numeric: true, width: 96, value: (r) => r.he.value ?? -1, render: (r) => mapCell(r.he) },
  {
    key: 'fire',
    label: 'Урон огнём',
    title: 'Molotov и Incendiary вместе',
    numeric: true,
    width: 112,
    value: (r) => r.fire.value ?? -1,
    render: (r) => mapCell(r.fire),
  },
  {
    key: 'flashed',
    label: 'Ослеплений',
    title: 'Пары «флешка — противник»: один противник может быть ослеплён несколько раз',
    numeric: true,
    width: 112,
    value: (r) => r.flashed.value ?? -1,
    render: (r) => mapCell(r.flashed),
  },
  { key: 'smokes', label: 'Смоков', numeric: true, width: 84, value: (r) => r.smokes.value ?? -1, render: (r) => mapCell(r.smokes) },
  {
    key: 'kills',
    label: 'Убийства',
    title: 'Убийства HE и огнём (Molotov и Incendiary) за период',
    numeric: true,
    width: 96,
    value: (r) => r.kills.value ?? -1,
    render: (r) => mapCell(r.kills, fmtInt),
  },
]

export function UtilityTab({ steamId, query, sessions }: TabProps) {
  const load = useExtLoad<PlayerUtility>(() => api.getPlayerUtility(steamId, query), `${steamId}|${query}`)
  return (
    <ExtTab load={load} sessions={sessions}>
      {(d: PlayerUtility) => {
        const g = d.grenades
        const list = [g.he, g.fire, g.flashed, g.smokes, g.grenadeKills]
        const uniform = sameCoverage(list)
        const b = d.bomb
        return (
          <>
            <Card aria-labelledby="gr-h">
              <Header
                id="gr-h"
                title="Гранаты"
                note="в среднем за карту"
                aside={
                  <Muted>
                    {uniform
                      ? g.he.covered === g.he.total
                        ? `по ${plural(g.he.covered, ['карте', 'картам', 'картам'])}`
                        : `по ${g.he.covered} картам из ${g.he.total}`
                      : 'покрытие у показателей разное — см. под числом'}
                  </Muted>
                }
              />
              <KpiGrid cols={6}>
                <GrenadeKpi label="Урон HE" metric={g.he} sub="за карту" uniform={uniform} />
                <GrenadeKpi label="Урон огнём" metric={g.fire} sub="Molotov и Incendiary" uniform={uniform} />
                <GrenadeKpi label="Ослеплений противников" metric={g.flashed} sub="пары «флешка — противник»" uniform={uniform} />
                <Kpi
                  label="Секунды ослепления"
                  value={<Checking title="Метрика выводится только после проверки алгоритма" />}
                  sub="появятся после проверки"
                />
                <GrenadeKpi label="Смоков брошено" metric={g.smokes} sub="за карту" uniform={uniform} />
                <Kpi
                  label="Убийства гранатами"
                  value={<MetricNum metric={{ ...g.grenadeKills, value: g.grenadeKills.count }} format={fmtInt} />}
                  sub={`за период: HE ${g.grenadeKills.he}, огонь ${g.grenadeKills.fire}`}
                  coverage={
                    !uniform &&
                    coverageNote(g.grenadeKills) && (
                      <CoverageChip title={missingTip(g.grenadeKills.missing)}>{coverageNote(g.grenadeKills)}</CoverageChip>
                    )
                  }
                />
              </KpiGrid>
              <div className="flex flex-wrap items-baseline gap-3 border-t border-border px-4 pt-4 pb-3 md:px-5">
                <h3 className="m-0 text-sub" id="gm-h">
                  По картам
                </h3>
                <Muted>за каждую карту целиком</Muted>
              </div>
              <StatTable columns={GRENADE_MAP_COLUMNS} rows={d.byMap} rowKey={(r) => r.map} labelledBy="gm-h" minWidth={640} />
              <CardFooter className="text-small text-fg-muted">
                Убийства гранатами — сумма за период, остальное — среднее за карту. Ослепления — пары «флешка — противник»,
                а не разные люди: одного противника можно ослепить несколько раз. Секунды ослепления появятся, когда
                алгоритм пройдёт проверку.
              </CardFooter>
            </Card>

            <Card aria-labelledby="bm-h">
              <Header id="bm-h" title="Бомба" note="за период" metric={b} />
              <KpiGrid cols={4}>
                <Kpi label="Постановки" value={b.plants} sub="успешно заложил" />
                <Kpi label="Начато постановок" value={b.plantStarts} sub={`прервано ${b.plantsAborted}`} />
                <Kpi label="Дефьюзы" value={b.defuses} sub="успешно разминировал" />
                <Kpi label="Начато дефьюзов" value={b.defuseStarts} sub={`прервано ${b.defusesAborted}`} />
              </KpiGrid>
            </Card>
          </>
        )
      }}
    </ExtTab>
  )
}
