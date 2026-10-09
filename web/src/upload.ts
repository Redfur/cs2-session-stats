// Очередь загрузки демок: файлы отправляются строго по одному в порядке очереди,
// поэтому порядок матчей совпадает с порядком файлов.

import { useCallback, useEffect, useRef, useState } from 'react'
import { api, type UploadResult } from './api'

// Поддерживаемые форматы, как ingest.detectFormat на сервере: суффикс без учёта регистра.
export const DEMO_FORMATS = ['.dem', '.dem.gz', '.dem.bz2', '.dem.zst', '.zip']

export function isSupportedDemo(name: string): boolean {
  const lower = name.toLowerCase()
  return DEMO_FORMATS.some((f) => lower.endsWith(f))
}

export type UploadState = 'queued' | 'sending' | 'accepted' | 'duplicate' | 'error' | 'cancelled'

export interface UploadItem {
  id: number
  file: File
  state: UploadState
  loaded: number
  total: number
  result?: UploadResult
  error?: string
  // оценка оставшегося времени, секунды; появляется через 2 с после начала отправки
  eta?: number
}

export function isPending(it: UploadItem): boolean {
  return it.state === 'queued' || it.state === 'sending'
}

// Скорость сглаживается экспонентой с постоянной времени в несколько секунд.
const SPEED_TAU_MS = 3000
const ETA_DELAY_MS = 2000

// useUploadQueue хранит очередь загрузки сессии. onAccepted вызывается после каждого принятого файла.
export function useUploadQueue(sessionId: string, onAccepted: () => void) {
  const [items, setItems] = useState<UploadItem[]>([])
  // имена файлов, отсеянных по формату при последнем добавлении
  const [rejected, setRejected] = useState<string[]>([])
  // очередь ждущих файлов и отправляемый сейчас — источник правды для порядка отправки;
  // items в состоянии — их отображение
  const queue = useRef<UploadItem[]>([])
  const active = useRef<{ id: number; ctrl: AbortController } | null>(null)
  const nextId = useRef(1)
  const onAcceptedRef = useRef(onAccepted)
  useEffect(() => {
    onAcceptedRef.current = onAccepted
  }, [onAccepted])

  const patch = useCallback((id: number, change: Partial<UploadItem>) => {
    setItems((prev) => prev.map((it) => (it.id === id ? { ...it, ...change } : it)))
  }, [])

  // pump запускает следующий файл, если сейчас ничего не отправляется
  const pump = useCallback(
    function pumpNext() {
      if (active.current) return
      const item = queue.current.shift()
      if (!item) return
      const ctrl = new AbortController()
      active.current = { id: item.id, ctrl }
      patch(item.id, { state: 'sending', loaded: 0, total: item.file.size })

      const t0 = performance.now()
      let lastT = t0
      let lastLoaded = 0
      let speed = 0 // байт в секунду
      const onProgress = (loaded: number, total: number) => {
        const now = performance.now()
        const dt = now - lastT
        if (dt > 0) {
          const inst = ((loaded - lastLoaded) / dt) * 1000
          speed = speed === 0 ? inst : speed + (1 - Math.exp(-dt / SPEED_TAU_MS)) * (inst - speed)
          lastT = now
          lastLoaded = loaded
        }
        const eta = now - t0 >= ETA_DELAY_MS && speed > 0 ? (total - loaded) / speed : undefined
        patch(item.id, { loaded, total, eta })
      }

      api
        .uploadDemo(sessionId, item.file, onProgress, ctrl.signal)
        .then(
          (res) => {
            patch(item.id, { state: res.status, result: res, error: res.error })
            if (res.status === 'accepted') onAcceptedRef.current()
          },
          (err: Error) => {
            patch(item.id, err.name === 'AbortError' ? { state: 'cancelled' } : { state: 'error', error: err.message })
          },
        )
        .finally(() => {
          // после ухода со страницы очередь не продолжается
          if (active.current?.ctrl !== ctrl) return
          active.current = null
          pumpNext()
        })
    },
    [sessionId, patch],
  )

  // при уходе со страницы незаконченные загрузки обрываются
  useEffect(
    () => () => {
      queue.current = []
      const a = active.current
      active.current = null
      a?.ctrl.abort()
    },
    [],
  )

  const add = useCallback(
    (files: File[]) => {
      const ok = files.filter((f) => isSupportedDemo(f.name))
      setRejected(files.filter((f) => !isSupportedDemo(f.name)).map((f) => f.name))
      if (ok.length === 0) return
      const added: UploadItem[] = ok.map((file) => ({ id: nextId.current++, file, state: 'queued', loaded: 0, total: file.size }))
      const idle = !active.current && queue.current.length === 0
      queue.current.push(...added)
      // итоги прошлой загрузки висят до следующей
      setItems((prev) => [...(idle ? [] : prev), ...added])
      pump()
    },
    [pump],
  )

  // cancel убирает файл из очереди или обрывает его отправку
  const cancel = useCallback((id: number) => {
    if (active.current?.id === id) {
      active.current.ctrl.abort()
      return
    }
    queue.current = queue.current.filter((it) => it.id !== id)
    setItems((prev) => prev.filter((it) => it.id !== id))
  }, [])

  // cancelAll отменяет текущий файл и оставшуюся очередь; принятые файлы не затрагиваются
  const cancelAll = useCallback(() => {
    const ids = new Set(queue.current.map((it) => it.id))
    queue.current = []
    setItems((prev) => prev.filter((it) => !ids.has(it.id)))
    active.current?.ctrl.abort()
  }, [])

  const clear = useCallback(() => {
    setItems((prev) => prev.filter(isPending))
    setRejected([])
  }, [])

  return { items, rejected, add, cancel, cancelAll, clear }
}
