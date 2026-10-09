import type { SetURLSearchParams } from 'react-router'

// Изменяет копию параметров фильтра и записывает её в URL; каждое изменение — новая запись истории.
export function updateParams(params: URLSearchParams, setParams: SetURLSearchParams, change: (p: URLSearchParams) => void) {
  const next = new URLSearchParams(params)
  change(next)
  setParams(next)
}
