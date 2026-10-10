import type { WeaponRow } from '../api'
import { fmtInt, fmtPct } from '../ext'
import { NotComputed } from './ui/ExtValue'
import type { Column } from './ui/StatTable'
import { WeaponLabel } from './ui/WeaponIcon'

// Колонки таблицы оружия: профиль и матч. Без событий урона урон и попадания не посчитаны.
export function weaponColumns(damageCovered: boolean): Column<WeaponRow>[] {
  const noDamage = 'В демках нет событий урона — не посчитано'
  return [
    { key: 'weapon', label: 'Оружие', value: (r) => r.weapon, render: (r) => <WeaponLabel code={r.weapon} /> },
    { key: 'kills', label: 'Убийства', numeric: true, width: 96, value: (r) => r.kills },
    {
      key: 'hsk',
      label: 'HS% убийств',
      title: 'Доля убийств в голову среди убийств этим оружием',
      numeric: true,
      width: 112,
      value: (r) => r.hsKillsPct ?? -1,
      render: (r) =>
        r.hsKillsPct == null ? <NotComputed reason="Нет убийств этим оружием — долю не считаем" /> : fmtPct(r.hsKillsPct),
    },
    {
      key: 'dmg',
      label: 'Урон',
      numeric: true,
      secondary: true,
      width: 84,
      value: (r) => r.damage,
      render: (r) => (damageCovered ? fmtInt(r.damage) : <NotComputed reason={noDamage} />),
    },
    { key: 'shots', label: 'Выстрелы', numeric: true, secondary: true, width: 96, value: (r) => r.shots },
    {
      key: 'hits',
      label: 'Попадания',
      numeric: true,
      secondary: true,
      width: 104,
      value: (r) => r.hits,
      render: (r) => (damageCovered ? r.hits : <NotComputed reason={noDamage} />),
    },
    {
      key: 'hsh',
      label: 'HS% попаданий',
      title: 'Доля попаданий в голову среди всех попаданий',
      numeric: true,
      width: 128,
      value: (r) => r.hsHitsPct ?? -1,
      render: (r) =>
        !damageCovered ? (
          <NotComputed reason={noDamage} />
        ) : r.hsHitsPct == null ? (
          <NotComputed reason="Нет попаданий — долю не считаем" />
        ) : (
          fmtPct(r.hsHitsPct)
        ),
    },
  ]
}
