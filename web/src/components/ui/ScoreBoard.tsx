import type { ReactNode } from 'react'
import { OutcomeBadge } from './Badge'
import { Card } from './Card'
import { Score, TeamName } from './Score'

function outcome(own: number, other: number) {
  return own > other ? 'win' : own < other ? 'loss' : 'draw'
}

interface ScoreBoardProps {
  scoreA: number
  scoreB: number
  // длительность матча под счётом и подсказка к ней
  duration?: { text: string; title: string }
  footer?: ReactNode
  // содержимое карточки под счётом: полоса раундов
  children?: ReactNode
}

// ScoreBoard — крупный счёт матча. Победившая сторона отмечена бейджем «Победа», а не только цветом.
export function ScoreBoard({ scoreA, scoreB, duration, footer, children }: ScoreBoardProps) {
  const label =
    `${scoreA} : ${scoreB}, ` +
    (scoreA === scoreB ? 'ничья' : `победа команды ${scoreA > scoreB ? 'A' : 'B'}`)
  return (
    <Card aria-label="Счёт матча">
      <div className="px-4 pt-6 md:px-8 md:pt-8">
        <div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-3 md:gap-7">
          <div className="flex min-w-0 flex-col items-start gap-2.5">
            <TeamName team="A" className="text-[14px] leading-6 md:text-[18px] [&>span:first-child]:md:size-3" />
            <OutcomeBadge result={outcome(scoreA, scoreB)} />
          </div>
          <div className="flex flex-col items-center gap-1.5">
            <div aria-label={label} role="img">
              <Score a={scoreA} b={scoreB} className="text-[40px] leading-10 font-extrabold tracking-[-0.02em] md:text-score" />
            </div>
            {duration && (
              <span className="text-small text-fg-muted tabular-nums" title={duration.title}>
                <span className="sr-only">Длительность матча: </span>
                {duration.text}
              </span>
            )}
          </div>
          <div className="flex min-w-0 flex-col items-end gap-2.5">
            <TeamName team="B" className="text-[14px] leading-6 md:text-[18px] [&>span:last-child]:md:size-3" />
            <OutcomeBadge result={outcome(scoreB, scoreA)} />
          </div>
        </div>
      </div>
      {children}
      {footer && (
        <div className="mx-4 mt-6 border-t border-border pt-3.5 pb-4 text-center text-small text-fg-muted tabular-nums md:mx-8">
          {footer}
        </div>
      )}
    </Card>
  )
}
