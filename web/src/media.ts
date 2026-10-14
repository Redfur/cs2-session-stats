// Узкий экран макета — до 720 px (--breakpoint-md: 45rem). Нужен там, где на узком экране
// таблица показывает другой набор колонок, а не прячет их стилями.

import { useEffect, useState } from 'react'

const NARROW = '(max-width: 44.999rem)'

export function useNarrow(): boolean {
  const [narrow, setNarrow] = useState(() => window.matchMedia(NARROW).matches)
  useEffect(() => {
    const mq = window.matchMedia(NARROW)
    const onChange = () => setNarrow(mq.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])
  return narrow
}
