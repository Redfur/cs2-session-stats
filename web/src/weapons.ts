// Справочник оружия: код из демки (internal/parser.WeaponCode) → название и иконка.
// Иконки — игровые SVG CS2 из Juknum/counter-strike-icons (cs2/panorama/images/icons/equipment).

const icons = import.meta.glob<string>('./assets/weapons/*.svg', { eager: true, query: '?url', import: 'default' })

const NAMES: Record<string, string> = {
  ak47: 'AK-47',
  m4a1: 'M4A4',
  m4a1_silencer: 'M4A1-S',
  galilar: 'Galil AR',
  famas: 'FAMAS',
  aug: 'AUG',
  sg556: 'SG 553',
  awp: 'AWP',
  ssg08: 'SSG 08',
  g3sg1: 'G3SG1',
  scar20: 'SCAR-20',
  mp9: 'MP9',
  mac10: 'MAC-10',
  mp7: 'MP7',
  mp5sd: 'MP5-SD',
  ump45: 'UMP-45',
  p90: 'P90',
  bizon: 'PP-Bizon',
  nova: 'Nova',
  xm1014: 'XM1014',
  sawedoff: 'Sawed-Off',
  mag7: 'MAG-7',
  negev: 'Negev',
  m249: 'M249',
  glock: 'Glock-18',
  usp_silencer: 'USP-S',
  hkp2000: 'P2000',
  p250: 'P250',
  fiveseven: 'Five-SeveN',
  tec9: 'Tec-9',
  cz75a: 'CZ75-Auto',
  deagle: 'Desert Eagle',
  revolver: 'R8 Revolver',
  elite: 'Dual Berettas',
  hegrenade: 'HE Grenade',
  flashbang: 'Flashbang',
  smokegrenade: 'Smoke Grenade',
  molotov: 'Molotov',
  incgrenade: 'Incendiary Grenade',
  decoy: 'Decoy Grenade',
  knife: 'Нож',
  taser: 'Zeus x27',
  c4: 'C4',
  inferno: 'Огонь',
  world: 'Мир',
}

export interface WeaponInfo {
  name: string
  // url иконки; нет — оружие не опознано
  icon?: string
  known: boolean
}

// weaponInfo возвращает название и иконку оружия. Неизвестный код — «Не опознано».
export function weaponInfo(code: string): WeaponInfo {
  const name = NAMES[code]
  if (!name) return { name: 'Не опознано', known: false }
  return { name, icon: icons[`./assets/weapons/${code}.svg`], known: true }
}
