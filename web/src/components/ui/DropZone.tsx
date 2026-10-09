import { Upload } from 'lucide-react'
import { useRef, useState, type DragEvent } from 'react'
import { plural } from '../../format'
import { DEMO_FORMATS } from '../../upload'
import { cx } from './cx'

interface DropZoneProps {
  // large — большая зона пустой сессии, strip — полоса под списком матчей
  variant: 'large' | 'strip'
  onFiles: (files: File[]) => void
  // номер, который получит первый новый матч
  nextNumber: number
  // файлы, отсеянные по формату
  rejected: string[]
}

function numbers(first: number, count: number): string {
  const list = Array.from({ length: Math.min(count, 4) }, (_, i) => `#${first + i}`)
  if (count > 4) return `${list.join(', ')} и дальше`
  if (list.length === 1) return list[0]
  return `${list.slice(0, -1).join(', ')} и ${list[list.length - 1]}`
}

// DropZone принимает демки перетаскиванием и через выбор файлов.
// Щелчок по зоне открывает выбор файлов; с клавиатуры — кнопка «выберите файлы» внутри.
export function DropZone({ variant, onFiles, nextNumber, rejected }: DropZoneProps) {
  const input = useRef<HTMLInputElement>(null)
  // число файлов над зоной; null — ничего не перетаскивают
  const [over, setOver] = useState<number | null>(null)
  const depth = useRef(0)

  const hasFiles = (e: DragEvent) => Array.from(e.dataTransfer.types).includes('Files')

  const handlers = {
    onDragEnter: (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
      depth.current++
      setOver(e.dataTransfer.items.length)
    },
    onDragOver: (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
      e.dataTransfer.dropEffect = 'copy'
    },
    onDragLeave: () => {
      depth.current = Math.max(0, depth.current - 1)
      if (depth.current === 0) setOver(null)
    },
    onDrop: (e: DragEvent) => {
      e.preventDefault()
      depth.current = 0
      setOver(null)
      onFiles(Array.from(e.dataTransfer.files))
    },
  }

  const pick = (
    <button
      type="button"
      className="focus-ring cursor-pointer rounded border-0 bg-transparent p-0 font-[inherit] font-bold text-accent underline underline-offset-3 hover:text-accent-hover"
      onClick={(e) => {
        e.stopPropagation()
        input.current?.click()
      }}
    >
      {variant === 'large' ? 'выберите их' : 'выберите файлы'}
    </button>
  )
  const fileInput = (
    <input
      ref={input}
      type="file"
      multiple
      hidden
      accept={DEMO_FORMATS.join(',') + ',.gz,.bz2,.zst'}
      onChange={(e) => {
        onFiles(Array.from(e.target.files ?? []))
        e.target.value = ''
      }}
    />
  )
  const icon = (
    <span
      className={cx(
        'grid flex-none place-items-center transition-colors',
        variant === 'large' ? 'mb-1.5 size-11 rounded-lg [&>svg]:size-[22px]' : 'size-9 rounded-[10px] [&>svg]:size-[18px]',
        over !== null ? 'bg-accent text-accent-fg' : 'bg-surface-2 text-fg-muted',
      )}
      aria-hidden
    >
      <Upload />
    </span>
  )
  const zone = cx(
    'cursor-pointer border-[1.5px] transition-colors',
    over !== null ? 'border-solid border-accent bg-accent/14' : 'border-dashed border-border-strong hover:border-border-hover',
  )
  const overText = over !== null && (
    <>
      Отпустите, чтобы загрузить {filesText(over)}. Станут матчами {numbers(nextNumber, over)} в этом порядке
    </>
  )

  return (
    <div className="flex flex-col gap-2">
      {variant === 'large' ? (
        <div
          {...handlers}
          onClick={() => input.current?.click()}
          className={cx(zone, 'flex min-h-[300px] flex-col items-center justify-center gap-1.5 rounded-lg bg-surface px-6 py-8 text-center')}
        >
          {icon}
          {over !== null ? (
            <>
              <p className="m-0 text-sub">Отпустите, чтобы загрузить {filesText(over)}</p>
              <p className="m-0 text-[13px] text-fg-muted">Станут матчами {numbers(nextNumber, over)} в этом порядке</p>
            </>
          ) : (
            <>
              <p className="m-0 text-sub">Загрузите демки вечера</p>
              <p className="m-0 max-w-[440px] text-[13px] leading-4 text-fg-muted">
                Перетащите файлы сюда или {pick}. Можно сразу несколько — порядок файлов станет порядком матчей.
              </p>
              <div className="mt-2 flex flex-wrap justify-center gap-1.5">
                {DEMO_FORMATS.map((f) => (
                  <span key={f} className="rounded-[5px] border border-border bg-surface-2 px-[7px] py-0.5 font-mono text-[11px] leading-4 text-fg-muted">
                    {f}
                  </span>
                ))}
              </div>
            </>
          )}
          {fileInput}
        </div>
      ) : (
        <div
          {...handlers}
          onClick={() => input.current?.click()}
          className={cx(zone, 'flex items-center gap-3.5 rounded-md px-4 py-3.5', over === null && 'hover:bg-surface-2')}
        >
          {icon}
          <div className="min-w-0">
            {over !== null ? (
              <div className="font-semibold">{overText}</div>
            ) : (
              <>
                <div className="font-semibold">Перетащите демки сюда или {pick}</div>
                <div className="text-small text-fg-muted">
                  {DEMO_FORMATS.join(', ')} · станут матчами #{nextNumber} и дальше в порядке файлов
                </div>
              </>
            )}
          </div>
          {fileInput}
        </div>
      )}
      {rejected.length > 0 && (
        <p className="m-0 text-small text-status-error" role="alert">
          {rejected.length === 1 ? `Файл ${rejected[0]} пропущен` : `Файлы ${rejected.join(', ')} пропущены`}: поддерживаются{' '}
          {DEMO_FORMATS.join(', ')}
        </p>
      )}
    </div>
  )
}

function filesText(n: number) {
  return plural(n, ['файл', 'файла', 'файлов'])
}
