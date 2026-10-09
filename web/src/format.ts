// Форматирование чисел, размеров и дат для интерфейса.

// plural выбирает форму слова по числу: plural(5, ['матч', 'матча', 'матчей']) → «5 матчей».
export function plural(n: number, forms: [string, string, string]): string {
  const a = n % 10
  const b = n % 100
  const form = a === 1 && b !== 11 ? forms[0] : a >= 2 && a <= 4 && (b < 12 || b > 14) ? forms[1] : forms[2]
  return `${n} ${form}`
}

const MB = 1024 * 1024
const GB = 1024 * MB

function unitOf(bytes: number) {
  return bytes >= GB ? { div: GB, name: 'ГБ', digits: 1 } : { div: MB, name: 'МБ', digits: 0 }
}

function num(x: number, digits: number) {
  return x.toLocaleString('ru', { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

export function formatBytes(bytes: number): string {
  const u = unitOf(bytes)
  return `${num(bytes / u.div, u.digits)} ${u.name}`
}

// formatBytesOf: «414 из 690 МБ» — обе величины в единицах полного объёма.
export function formatBytesOf(part: number, total: number): string {
  const u = unitOf(total)
  return `${num(part / u.div, u.digits)} из ${num(total / u.div, u.digits)} ${u.name}`
}

export function formatEta(seconds: number): string {
  if (seconds < 60) return `~${Math.max(1, Math.round(seconds))} с`
  if (seconds < 3600) return `~${Math.round(seconds / 60)} мин`
  return `~${(seconds / 3600).toFixed(1).replace('.', ',')} ч`
}

// ISO-дата ГГГГ-ММ-ДД → Date в локальной зоне без сдвига на часовой пояс.
function parseDate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

// «9 октября 2026»
export function formatDateLong(iso: string): string {
  return parseDate(iso).toLocaleDateString('ru', { day: 'numeric', month: 'long', year: 'numeric' }).replace(' г.', '')
}

// «09.10.2026»
export function formatDateShort(iso: string): string {
  return parseDate(iso).toLocaleDateString('ru', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

// «пт»
export function formatWeekday(iso: string): string {
  return parseDate(iso).toLocaleDateString('ru', { weekday: 'short' })
}

// «Октябрь 2026»
export function formatMonth(iso: string): string {
  const d = parseDate(iso)
  const month = new Intl.DateTimeFormat('ru', { month: 'long' }).format(d)
  return `${month[0].toUpperCase()}${month.slice(1)} ${d.getFullYear()}`
}
