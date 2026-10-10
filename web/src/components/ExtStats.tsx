import { Info, Swords } from 'lucide-react'
import type { ReactNode } from 'react'
import type { ExtHeader, ExtMetric } from '../api'
import { byMaps, missingTip, notComputedReason } from '../ext'
import { Alert } from './ui/Alert'
import { Badge } from './ui/Badge'
import { Button, ButtonLink } from './ui/Button'
import { Card, CardBody } from './ui/Card'
import { cx } from './ui/cx'
import { EmptyState } from './ui/EmptyState'
import { NotComputed } from './ui/ExtValue'

// MetricNum — число метрики или прочерк с причиной. noValue — подсказка, когда матчи покрыты,
// но у значения нет знаменателя.
export function MetricNum({
  metric,
  format,
  noValue = 'Нет данных — значение не считаем',
}: {
  metric: ExtMetric
  format: (v: number) => ReactNode
  noValue?: string
}) {
  if (metric.covered === 0) return <NotComputed reason={notComputedReason(metric)} />
  if (metric.value == null) return <NotComputed reason={noValue} />
  return <>{format(metric.value)}</>
}

// CoverageBadge — бейдж в заголовке карточки, когда покрытие у всего блока одинаковое и неполное.
export function CoverageBadge({ metric }: { metric: ExtMetric }) {
  if (metric.covered === metric.total || metric.covered === 0) return null
  return (
    <span title={missingTip(metric.missing)}>
      <Badge tone="neutral" icon={<Info aria-hidden />}>
        {byMaps(metric.covered)} из {metric.total}
      </Badge>
    </span>
  )
}

// ── Состояния блока ──

export function ExtLoading({ label }: { label: string }) {
  return (
    <div className="flex flex-col gap-6" aria-busy>
      <span className="sr-only" role="status">
        {label}
      </span>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
        {[0, 1].map((i) => (
          <SkeletonCard key={i} />
        ))}
      </div>
      <SkeletonCard />
    </div>
  )
}

function SkeletonCard() {
  const bar = 'block animate-pulse rounded-[4px] bg-surface-2 motion-reduce:animate-none'
  return (
    <Card as="div">
      <div className="border-b border-border px-4 py-4 md:px-5">
        <span className={cx(bar, 'h-[18px] w-28')} />
      </div>
      <CardBody className="flex flex-col gap-2.5">
        <span className={cx(bar, 'h-7 w-[45%]')} />
        <span className={cx(bar, 'h-3 w-[70%]')} />
      </CardBody>
    </Card>
  )
}

export function ExtError({ title, text, onRetry }: { title: string; text: string; onRetry: () => void }) {
  return (
    <Card as="div">
      <CardBody>
        <Alert
          tone="error"
          title={title}
          action={
            <Button size="sm" onClick={onRetry}>
              Повторить
            </Button>
          }
        >
          {text}
        </Alert>
      </CardBody>
    </Card>
  )
}

export function ExtNotComputed({ text, session }: { text: ReactNode; session?: { id: number; title: string } }) {
  return (
    <Card as="div">
      <EmptyState
        icon={<Swords />}
        as="h2"
        title="Подробная статистика ещё не посчитана"
        text={text}
        actions={
          session && (
            <ButtonLink to={`/sessions/${session.id}`} variant="secondary">
              К сессии {session.title}
            </ButtonLink>
          )
        }
      />
    </Card>
  )
}

const ordinals = (list: { ordinal: number }[]) =>
  list
    .map((m) => `#${m.ordinal}`)
    .join(', ')
    .replace(/, ([^,]*)$/, ' и $1')

// ExtAlerts — пересчёт и его ошибка под шапкой страницы: прежние данные остаются на месте.
export function ExtAlerts({ header, sessionLink }: { header: ExtHeader; sessionLink?: (id: number) => ReactNode }) {
  const partial = header.coveredMatches < header.eligibleMatches
  const kept = partial
    ? `Прежние данные сохранены: подробная статистика — ${byMaps(header.coveredMatches)} из ${header.eligibleMatches}.`
    : 'Прежние данные сохранены.'
  const word = (n: number) => (n === 1 ? 'матча' : 'матчей')
  return (
    <>
      {header.reparsing.length > 0 && (
        <Alert tone="info" title={`Идёт пересчёт ${word(header.reparsing.length)} ${ordinals(header.reparsing)}.`}>
          {partial
            ? `Пока на странице прежние данные: подробная статистика — ${byMaps(header.coveredMatches)} из ${header.eligibleMatches}.`
            : 'Пока на странице прежние данные.'}{' '}
          Обновится сама, когда пересчёт закончится.
        </Alert>
      )}
      {header.failed.length > 0 && (
        <Alert
          tone="error"
          title={`Пересчёт ${word(header.failed.length)} ${ordinals(header.failed)} не удался.`}
          action={sessionLink?.(header.failed[0].sessionId)}
        >
          {header.failed[0].error && `${header.failed[0].error.replace(/\.$/, '')}. `}
          {kept}
        </Alert>
      )}
    </>
  )
}
