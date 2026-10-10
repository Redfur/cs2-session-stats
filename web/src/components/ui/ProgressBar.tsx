import { cx } from './cx'

// ProgressBar — тонкая полоса прогресса (0–100); done — завершено, зелёная.
export function ProgressBar({ value, done, label }: { value: number; done?: boolean; label: string }) {
  return (
    <div
      className="mt-2 h-1 overflow-hidden rounded-xs bg-surface-hover"
      role="progressbar"
      aria-valuenow={Math.round(value)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
    >
      <span className={cx('block h-full rounded-xs', done ? 'bg-status-ok' : 'bg-status-progress')} style={{ width: `${value}%` }} />
    </div>
  )
}
