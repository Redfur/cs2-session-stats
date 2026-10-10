package parser

import (
	"strings"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

// Коды оружия — внутренние имена CS2 без префикса weapon_. Все ножи — knife.
const (
	WeaponUnknown    = "unknown"
	WeaponHE         = "hegrenade"
	WeaponMolotov    = "molotov"
	WeaponIncendiary = "incgrenade"
	WeaponFlash      = "flashbang"
	WeaponSmoke      = "smokegrenade"
	WeaponDecoy      = "decoy"
)

var weaponCodes = map[common.EquipmentType]string{
	common.EqP2000:        "hkp2000",
	common.EqGlock:        "glock",
	common.EqP250:         "p250",
	common.EqDeagle:       "deagle",
	common.EqFiveSeven:    "fiveseven",
	common.EqDualBerettas: "elite",
	common.EqTec9:         "tec9",
	common.EqCZ:           "cz75a",
	common.EqUSP:          "usp_silencer",
	common.EqRevolver:     "revolver",
	common.EqMP7:          "mp7",
	common.EqMP9:          "mp9",
	common.EqBizon:        "bizon",
	common.EqMac10:        "mac10",
	common.EqUMP:          "ump45",
	common.EqP90:          "p90",
	common.EqMP5:          "mp5sd",
	common.EqSawedOff:     "sawedoff",
	common.EqNova:         "nova",
	common.EqMag7:         "mag7",
	common.EqXM1014:       "xm1014",
	common.EqM249:         "m249",
	common.EqNegev:        "negev",
	common.EqGalil:        "galilar",
	common.EqFamas:        "famas",
	common.EqAK47:         "ak47",
	common.EqM4A4:         "m4a1",
	common.EqM4A1:         "m4a1_silencer",
	common.EqSSG08:        "ssg08",
	common.EqSG553:        "sg556",
	common.EqAUG:          "aug",
	common.EqAWP:          "awp",
	common.EqScar20:       "scar20",
	common.EqG3SG1:        "g3sg1",
	common.EqZeus:         "taser",
	common.EqKnife:        "knife",
	common.EqBomb:         "c4",
	common.EqWorld:        "world",
	common.EqDecoy:        WeaponDecoy,
	common.EqMolotov:      WeaponMolotov,
	common.EqIncendiary:   WeaponIncendiary,
	common.EqFlash:        WeaponFlash,
	common.EqSmoke:        WeaponSmoke,
	common.EqHE:           WeaponHE,
}

// WeaponCode возвращает код оружия. Для неизвестного типа — исходное имя из демки
// (raw, если оно есть), иначе WeaponUnknown.
func WeaponCode(eq *common.Equipment, raw string) string {
	if eq != nil {
		if code, ok := weaponCodes[eq.Type]; ok {
			return code
		}
	}
	raw = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "weapon_")
	switch {
	case raw == "":
		return WeaponUnknown
	case strings.HasPrefix(raw, "knife") || strings.HasPrefix(raw, "bayonet"):
		return "knife"
	case raw == "inferno":
		return WeaponIncendiary
	}
	return raw
}

// IsGrenade — граната или огонь от неё.
func IsGrenade(code string) bool {
	switch code {
	case WeaponHE, WeaponMolotov, WeaponIncendiary, WeaponFlash, WeaponSmoke, WeaponDecoy:
		return true
	}
	return false
}

// IsFire — огонь Molotov или Incendiary.
func IsFire(code string) bool { return code == WeaponMolotov || code == WeaponIncendiary }
