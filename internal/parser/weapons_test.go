package parser

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

func TestWeaponCode(t *testing.T) {
	cases := []struct {
		eq   *common.Equipment
		raw  string
		want string
	}{
		{&common.Equipment{Type: common.EqAK47}, "", "ak47"},
		{&common.Equipment{Type: common.EqM4A1}, "", "m4a1_silencer"},
		{&common.Equipment{Type: common.EqM4A4}, "", "m4a1"},
		{&common.Equipment{Type: common.EqP2000}, "", "hkp2000"},
		{&common.Equipment{Type: common.EqKnife}, "", "knife"},
		{&common.Equipment{Type: common.EqIncendiary}, "", WeaponIncendiary},
		{&common.Equipment{Type: common.EqHE}, "", WeaponHE},
		{&common.Equipment{Type: common.EqUnknown}, "weapon_knife_karambit", "knife"},
		{&common.Equipment{Type: common.EqUnknown}, "bayonet", "knife"},
		{&common.Equipment{Type: common.EqUnknown}, "inferno", WeaponIncendiary},
		{&common.Equipment{Type: common.EqUnknown}, "weapon_xyz", "xyz"},
		{&common.Equipment{Type: common.EqUnknown}, "", WeaponUnknown},
		{nil, "", WeaponUnknown},
	}
	for _, c := range cases {
		if got := WeaponCode(c.eq, c.raw); got != c.want {
			t.Errorf("WeaponCode(%v, %q) = %q, ожидалось %q", c.eq, c.raw, got, c.want)
		}
	}
	for _, code := range []string{WeaponHE, WeaponMolotov, WeaponIncendiary, WeaponFlash, WeaponSmoke, WeaponDecoy} {
		if !IsGrenade(code) {
			t.Errorf("%s должен быть гранатой", code)
		}
	}
	if IsGrenade("ak47") || IsGrenade("knife") {
		t.Error("оружие не граната")
	}
	if !IsFire(WeaponMolotov) || !IsFire(WeaponIncendiary) || IsFire(WeaponHE) {
		t.Error("огонь определён неверно")
	}
}
