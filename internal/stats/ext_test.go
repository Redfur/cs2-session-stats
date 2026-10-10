package stats

import (
	"testing"
	"time"

	"cs2stats/internal/parser"
)

// Для расширенной статистики — команды по три игрока: a1, a2, a3 против b1, b2, b3.
const (
	a3 uint64 = 3
	b3 uint64 = 13
)

func players3() []parser.Player {
	return append(players(),
		parser.Player{SteamID: a3, Name: "a3", Team: parser.TeamA},
		parser.Player{SteamID: b3, Name: "b3", Team: parser.TeamB})
}

// round — раунд с началом в 1 с и концом end, команда A за CT.
func round(end float64, winner parser.Team, kills ...parser.Kill) parser.Round {
	return parser.Round{Number: 1, Winner: winner, SideA: parser.SideCT, Start: sec(1), End: sec(end), Reason: parser.ReasonElimination, Kills: kills}
}

func ext(rounds ...parser.Round) (Ext, map[uint64]ExtCounters) {
	for i := range rounds {
		if rounds[i].Number == 1 {
			rounds[i].Number = i + 1
		}
	}
	e := ComputeExt(parser.Match{Players: players3(), Rounds: rounds, HasDamageEvents: true, HasFlashEvents: true})
	by := map[uint64]ExtCounters{}
	for _, p := range e.Players {
		by[p.SteamID] = p.ExtCounters
	}
	return e, by
}

func TestExtTradeWindow(t *testing.T) {
	_, c := ext(round(100, parser.TeamA, kill(10, b1, a1), kill(15, a2, b1)))
	if c[a1].TradedDeaths != 1 || c[a2].TradeKills != 1 {
		t.Errorf("ровно 5 с — размен: разменяли %d, в размен %d", c[a1].TradedDeaths, c[a2].TradeKills)
	}
	_, c = ext(round(100, parser.TeamA, kill(10, b1, a1), kill(15.1, a2, b1)))
	if c[a1].TradedDeaths != 0 || c[a2].TradeKills != 0 {
		t.Errorf("5,1 с — не размен: разменяли %d, в размен %d", c[a1].TradedDeaths, c[a2].TradeKills)
	}
}

func TestExtOneKillTradesTwoDeaths(t *testing.T) {
	e, c := ext(round(100, parser.TeamB, kill(10, b1, a1), kill(11, b1, a2), kill(14, a3, b1)))
	if c[a1].TradedDeaths != 1 || c[a2].TradedDeaths != 1 || c[a3].TradeKills != 1 {
		t.Errorf("разменяли a1=%d a2=%d, в размен a3=%d", c[a1].TradedDeaths, c[a2].TradedDeaths, c[a3].TradeKills)
	}
	if !e.Kills[2].Trade || e.Kills[0].Trade {
		t.Errorf("метка размена в ходе раунда: %+v", e.Kills)
	}
}

func TestExtPostRoundKill(t *testing.T) {
	// раунд закончился на 20 с, a1 убил b3 после конца: b1 остался бы один только после раунда
	e, c := ext(round(20, parser.TeamA, kill(10, a1, b2), kill(25, a1, b3)))
	if c[b1].SurvivedT != 1 || c[b1].RoundsT != 1 {
		t.Errorf("b1 дожил до конца раунда: %+v", c[b1])
	}
	if len(e.Clutches) != 0 {
		t.Errorf("постраундовое убийство не создаёт клатч: %+v", e.Clutches)
	}
	var weaponKills int
	for _, w := range e.Weapons {
		if w.SteamID == a1 {
			weaponKills += w.Kills
		}
	}
	if weaponKills != 2 {
		t.Errorf("убийства a1 по оружию %d, ожидалось 2 — как K", weaponKills)
	}
}

func TestExtInvariants(t *testing.T) {
	he := kill(10, a1, b1)
	he.Weapon = parser.WeaponHE
	flash := kill(12, a1, b2)
	flash.Assister, flash.FlashAssist, flash.Weapon = a2, true, "ak47"
	dmg := kill(14, b3, a3)
	dmg.Assister, dmg.Weapon = b1, "awp"
	rounds := []parser.Round{round(100, parser.TeamA, he, flash, dmg, kill(20, a1, a2))} // тимкилл
	m := parser.Match{Players: players3(), Rounds: rounds}
	e := ComputeExt(m)
	base := map[uint64]PlayerStats{}
	for _, p := range Compute(m) {
		base[p.SteamID] = p
	}
	weaponKills := map[uint64]int{}
	for _, w := range e.Weapons {
		weaponKills[w.SteamID] += w.Kills
	}
	for _, p := range e.Players {
		if got := weaponKills[p.SteamID] + p.GrenadeKills(); got != base[p.SteamID].Kills {
			t.Errorf("%d: оружие+гранаты %d, K %d", p.SteamID, got, base[p.SteamID].Kills)
		}
		if got := p.FlashAssists + p.DamageAssists; got != base[p.SteamID].Assists {
			t.Errorf("%d: ассисты %d, A %d", p.SteamID, got, base[p.SteamID].Assists)
		}
	}
	by := map[uint64]ExtCounters{}
	for _, p := range e.Players {
		by[p.SteamID] = p.ExtCounters
	}
	if by[a1].HEKills != 1 || by[a2].FlashAssists != 1 || by[b1].DamageAssists != 1 {
		t.Errorf("HE a1=%d, флеш a2=%d, обычный b1=%d", by[a1].HEKills, by[a2].FlashAssists, by[b1].DamageAssists)
	}
}

func TestExtDamage(t *testing.T) {
	r := round(100, parser.TeamA)
	r.Damages = []parser.Damage{
		{Attacker: a1, Victim: b1, Amount: 80, Weapon: "ak47", Head: true},
		{Attacker: a1, Victim: b1, Amount: 20, Weapon: "ak47"},
		{Attacker: a1, Victim: b2, Amount: 30, Weapon: parser.WeaponHE},
		{Attacker: a1, Victim: b2, Amount: 15, Weapon: parser.WeaponMolotov}, // огонь, пока a1 с винтовкой в руках
		{Attacker: b1, Victim: a1, Amount: 50, Weapon: "awp"},
		{Attacker: a2, Victim: a1, Amount: 20, Weapon: "ak47"}, // союзник
		{Attacker: 0, Victim: a1, Amount: 10},                  // мир
	}
	ak := parser.Shot{Shooter: a1, Weapon: "ak47"}
	r.Shots = []parser.Shot{ak, ak, ak, {Shooter: a1, Weapon: parser.WeaponHE}}
	e, c := ext(r)
	if c[a1].DamageDealt != 145 || c[a1].DamageTaken != 50 {
		t.Errorf("a1 нанёс %d, получил %d; ожидалось 145 и 50", c[a1].DamageDealt, c[a1].DamageTaken)
	}
	if c[a1].HEDamage != 30 || c[a1].FireDamage != 15 {
		t.Errorf("HE %d, огонь %d", c[a1].HEDamage, c[a1].FireDamage)
	}
	var akStats WeaponStats
	for _, w := range e.Weapons {
		if w.SteamID == a1 {
			if w.Weapon != "ak47" {
				t.Errorf("лишнее оружие в таблице: %+v", w)
			}
			akStats = w
		}
	}
	if akStats.Damage != 100 || akStats.Hits != 2 || akStats.HSHits != 1 || akStats.Shots != 3 {
		t.Errorf("AK-47: %+v", akStats)
	}
}

func TestExtRestoredRound(t *testing.T) {
	restored := parser.Round{Number: 2, Winner: parser.TeamA, SideA: parser.SideCT, Restored: true,
		Kills: []parser.Kill{kill(110, b1, a2), kill(111, b2, a3)}}
	e, c := ext(round(100, parser.TeamA, kill(10, a1, b1)), restored)
	if c[a1].Rounds != 1 {
		t.Errorf("раундов с расширенными данными %d, ожидалось 1", c[a1].Rounds)
	}
	if !e.RestoredRound || len(e.Rounds) != 2 || !e.Rounds[1].Restored {
		t.Errorf("восстановленный раунд в хронологии: %+v", e.Rounds)
	}
	for _, k := range e.Kills {
		if k.Round == 2 {
			t.Errorf("ход восстановленного раунда не сохраняется: %+v", k)
		}
	}
	if len(e.Clutches) != 1 || e.Clutches[0].Outcome != ClutchUnknown || e.Clutches[0].SteamID != a1 {
		t.Errorf("клатч в восстановленном раунде: %+v", e.Clutches)
	}
}

func TestExtClutches(t *testing.T) {
	cases := []struct {
		name    string
		round   parser.Round
		want    Clutch
		attempt int
	}{
		{"1v3 → 1v1 — одна попытка", round(100, parser.TeamA,
			kill(10, b1, a2), kill(12, b2, a3), kill(15, a1, b1), kill(18, a1, b2), kill(20, a1, b3)),
			Clutch{SteamID: a1, Vs: 3, Outcome: ClutchWin}, 1},
		{"победа бомбой", round(100, parser.TeamA,
			kill(10, b1, a2), kill(12, b2, a3), kill(15, a1, b1), kill(18, b2, a1)),
			Clutch{SteamID: a1, Vs: 3, Outcome: ClutchWin}, 1},
		{"поражение", round(100, parser.TeamB, kill(10, b1, a2), kill(12, b2, a3), kill(18, b2, a1)),
			Clutch{SteamID: a1, Vs: 3, Outcome: ClutchLoss}, 1},
		{"одновременные смерти", round(100, parser.TeamA,
			kill(10, b1, a2), kill(10, b2, a3), kill(10, a1, b1)),
			Clutch{SteamID: a1, Vs: 2, Outcome: ClutchWin}, 1},
	}
	for _, c := range cases {
		e, _ := ext(c.round)
		var mine []Clutch // попытки бывают у обеих команд; проверяем a1
		for _, cl := range e.Clutches {
			if cl.SteamID == a1 {
				mine = append(mine, cl)
			}
		}
		if len(mine) != c.attempt {
			t.Errorf("%s: попыток %d: %+v", c.name, len(mine), e.Clutches)
			continue
		}
		got := mine[0]
		got.Round = 0
		if got != c.want {
			t.Errorf("%s: %+v, ожидалось %+v", c.name, got, c.want)
		}
	}
}

func TestExtSurvival(t *testing.T) {
	r1 := round(61, parser.TeamB, kill(31, b1, a1)) // a1 за CT погиб через 30 с
	r2 := round(91, parser.TeamA)                   // a1 за T дожил до конца
	r2.SideA = parser.SideT
	r2.Kills = []parser.Kill{kill(95, b1, a1)} // смерть после конца раунда
	_, c := ext(r1, r2)
	got := c[a1]
	if got.RoundsCT != 1 || got.SurvivedCT != 0 || got.AliveMsCT != 30000 {
		t.Errorf("CT: %+v", got)
	}
	if got.RoundsT != 1 || got.SurvivedT != 1 || got.AliveMsT != 90000 {
		t.Errorf("T: %+v", got)
	}
}

func TestExtUtilityAndBomb(t *testing.T) {
	r := round(100, parser.TeamA)
	r.Throws = []parser.Throw{
		{Thrower: a1, Weapon: parser.WeaponSmoke, Projectile: 1},
		{Thrower: a1, Weapon: parser.WeaponSmoke, Projectile: 1}, // повтор события
		{Thrower: a1, Weapon: parser.WeaponSmoke, Projectile: 2},
		{Thrower: a1, Weapon: parser.WeaponFlash, Projectile: 3},
	}
	r.Flashes = []parser.Flash{
		{Attacker: a1, Victim: b1, Projectile: 3, Duration: time.Second},
		{Attacker: a1, Victim: b1, Projectile: 3, Duration: time.Second}, // повтор пары
		{Attacker: a1, Victim: b1, Projectile: 4, Duration: time.Second}, // вторая флешка по тому же
		{Attacker: a1, Victim: b2, Projectile: 4},                        // без ослепления
		{Attacker: a1, Victim: a2, Projectile: 4, Duration: time.Second}, // союзник
	}
	r.Bomb = []parser.BombEvent{
		{Action: parser.BombPlantBegin, Player: b1},
		{Action: parser.BombPlantBegin, Player: b1},
		{Action: parser.BombPlanted, Player: b1},
		{Action: parser.BombDefuseBegin, Player: a2},
		{Action: parser.BombDefused, Player: a2},
	}
	_, c := ext(r)
	if c[a1].Smokes != 2 || c[a1].Flashed != 2 {
		t.Errorf("смоков %d, ослеплений %d; ожидалось 2 и 2", c[a1].Smokes, c[a1].Flashed)
	}
	if c[b1].PlantStarts != 2 || c[b1].Plants != 1 || c[a2].DefuseStarts != 1 || c[a2].Defuses != 1 {
		t.Errorf("бомба: b1 %+v, a2 %+v", c[b1], c[a2])
	}
}

func TestExtKillDetails(t *testing.T) {
	k := kill(10, a1, b1)
	k.ThroughSmoke, k.Penetrated, k.Distance, k.Weapon = true, 1, 1200, "awp"
	k2 := kill(12, a1, b2)
	k2.NoScope, k2.AttackerBlind, k2.Distance, k2.Weapon = true, true, 800, "awp"
	e, c := ext(round(100, parser.TeamA, k, k2))
	got := c[a1]
	if got.SmokeKills != 1 || got.WallbangKills != 1 || got.NoScopeKills != 1 || got.BlindKills != 1 {
		t.Errorf("детали убийств: %+v", got)
	}
	if got.DistanceKills != 2 || got.DistanceSum != 2000 {
		t.Errorf("дистанция %v по %d убийствам", got.DistanceSum, got.DistanceKills)
	}
	if !e.Kills[0].ThroughSmoke || !e.Kills[0].Wallbang {
		t.Errorf("флаги в ходе раунда: %+v", e.Kills[0])
	}
	if e.Duration != 99*time.Second {
		t.Errorf("длительность %v", e.Duration)
	}
}
