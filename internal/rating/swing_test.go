package rating

import (
	"math"
	"testing"
)

const fullBuy, eco = 23500, 4000

func TestWinProbT(t *testing.T) {
	in := func(name string, got, lo, hi float64) {
		t.Helper()
		if got < lo || got > hi {
			t.Errorf("%s: %.3f, ожидалось %.2f–%.2f", name, got, lo, hi)
		}
	}
	in("5v5", WinProbT(5, 5, false, fullBuy, fullBuy), 0.475, 0.485)
	in("5v4 — открывающее около +20 п.п.", WinProbT(5, 4, false, fullBuy, fullBuy)-WinProbT(5, 5, false, fullBuy, fullBuy), 0.15, 0.25)
	in("5v3 — двойное входное", WinProbT(5, 3, false, fullBuy, fullBuy), 0.84, 0.94)
	in("полное эко CT", WinProbT(5, 5, false, fullBuy, eco), 0.92, 0.98)
	if WinProbT(5, 5, true, fullBuy, fullBuy) <= WinProbT(5, 5, false, fullBuy, fullBuy) {
		t.Error("установленная бомба должна повышать шанс T")
	}
	if WinProbT(5, 5, false, 0, fullBuy) != WinProbT(5, 5, false, fullBuy, fullBuy) {
		t.Error("неизвестное снаряжение считается равным")
	}
	if WinProbT(3, 0, false, 0, 0) != 1 || WinProbT(0, 3, false, 0, 0) != 0 {
		t.Error("раунд без живых одной стороны уже решён")
	}
}

// команда A — CT, игроки A: 1–5, B: 11–15
func teams5() map[uint64]string {
	m := map[uint64]string{}
	for i := uint64(1); i <= 5; i++ {
		m[i], m[i+10] = "A", "B"
	}
	return m
}

func sum(m map[uint64]float64) float64 {
	var s float64
	for _, v := range m {
		s += v
	}
	return s
}

func TestOpeningKill(t *testing.T) {
	// T (команда B) убивает первым при равной закупке; раунд выигрывает B без других убийств
	r := SwingRound{Winner: "B", SideA: "CT", EquipA: fullBuy, EquipB: fullBuy, Kills: []SwingKill{{TimeMs: 1000, Killer: 11, Victim: 1}}}
	out := RoundSwing(r, teams5())
	kill := WinProbT(5, 4, false, fullBuy, fullBuy) - WinProbT(5, 5, false, fullBuy, fullBuy)
	if math.Abs(out[1]+kill) > 1e-9 || kill < 0.15 || kill > 0.25 {
		t.Errorf("жертва %v, прибавка за убийство %v (≈ +20 п.п.)", out[1], kill)
	}
	if math.Abs(sum(out)) > 1e-9 {
		t.Errorf("сумма по раунду %v", sum(out))
	}
}

func TestExitKill(t *testing.T) {
	// A ведёт 4 на 1 и добивает последнего
	r := SwingRound{Winner: "A", SideA: "CT", EquipA: fullBuy, EquipB: fullBuy}
	for i, v := range []uint64{11, 12, 13, 14} {
		r.Kills = append(r.Kills, SwingKill{TimeMs: int64(1000 * (i + 1)), Killer: 1, Victim: v})
	}
	r.Kills = append(r.Kills, SwingKill{TimeMs: 9000, Killer: 5, Victim: 5}) // самоубийство: 4 на 1
	r.Kills = append(r.Kills, SwingKill{TimeMs: 10000, Killer: 2, Victim: 15})
	before := 1 - WinProbT(1, 4, false, fullBuy, fullBuy)
	if gain := 1 - before; gain >= 0.03 {
		t.Errorf("добивание 4v1 даёт %.3f, ожидалось меньше 0.03", gain)
	}
	out := RoundSwing(r, teams5())
	if out[2] >= 0.03 {
		t.Errorf("Round Swing добившего %v", out[2])
	}
	if math.Abs(sum(out)) > 1e-9 {
		t.Errorf("сумма по раунду %v", sum(out))
	}
}

func TestClutch(t *testing.T) {
	// у A остался игрок 1 против четверых, дошёл до 1v1 с игроком 12 и выиграл
	r := SwingRound{Winner: "A", SideA: "CT", EquipA: fullBuy, EquipB: fullBuy}
	n := int64(0)
	kill := func(k, v uint64) {
		n++
		r.Kills = append(r.Kills, SwingKill{TimeMs: n * 1000, Killer: k, Victim: v})
	}
	for _, v := range []uint64{2, 3, 4, 5} {
		kill(12, v)
	}
	for _, v := range []uint64{11, 13, 14, 15} {
		kill(1, v) // 1v5 → 1v1
	}
	// победа в 1v1 переворачивает раунд примерно на половину
	if clutch := 1 - (1 - WinProbT(1, 1, false, fullBuy, fullBuy)); clutch < 0.45 || clutch > 0.55 {
		t.Errorf("победа в 1v1 стоит %.3f, ожидалось около 0.5", clutch)
	}
	kill(1, 12)
	out := RoundSwing(r, teams5())
	if math.Abs(sum(out)) > 1e-9 {
		t.Errorf("сумма по раунду %v", sum(out))
	}
	if out[1] < 0.5 {
		t.Errorf("клатчер получил %.3f за раунд", out[1])
	}
}

func TestEcoKillWorthMore(t *testing.T) {
	// одно и то же убийство: соперник в полной закупке и соперник на эко
	kill := func(equipVictim int) float64 {
		r := SwingRound{Winner: "B", SideA: "CT", EquipA: equipVictim, EquipB: fullBuy, Kills: []SwingKill{{TimeMs: 1000, Killer: 11, Victim: 1}}}
		return -RoundSwing(r, teams5())[1]
	}
	if full, e := kill(fullBuy), kill(eco); full <= e {
		t.Errorf("убийство против полной закупки %.3f должно стоить больше, чем против эко %.3f", full, e)
	}
}

func TestSavers(t *testing.T) {
	// A выиграл, у B до конца дожили 14 и 15 — остаток до поражения делится между ними
	r := SwingRound{Winner: "A", SideA: "CT", EquipA: fullBuy, EquipB: fullBuy}
	for i, v := range []uint64{11, 12, 13} {
		r.Kills = append(r.Kills, SwingKill{TimeMs: int64(1000 * (i + 1)), Killer: 1, Victim: v})
	}
	out := RoundSwing(r, teams5())
	rest := 1 - (1 - WinProbT(2, 5, false, fullBuy, fullBuy))
	if math.Abs(out[14]+rest/2) > 1e-9 || math.Abs(out[15]+rest/2) > 1e-9 {
		t.Errorf("сохранившие: %v, %v, ожидалось по %v", out[14], out[15], -rest/2)
	}
	if math.Abs(sum(out)) > 1e-9 {
		t.Errorf("сумма по раунду %v", sum(out))
	}
}

func TestAssistAndTeamKill(t *testing.T) {
	r := SwingRound{Winner: "B", SideA: "CT", EquipA: fullBuy, EquipB: fullBuy, PlantMs: 2500, Kills: []SwingKill{
		{TimeMs: 1000, Killer: 11, Victim: 1, Assister: 12},
		{TimeMs: 2000, Killer: 13, Victim: 14}, // убийство союзника
		{TimeMs: 3000, Victim: 2},              // смерть от мира после установки
	}}
	out := RoundSwing(r, teams5())
	gain := WinProbT(5, 4, false, fullBuy, fullBuy) - WinProbT(5, 5, false, fullBuy, fullBuy)
	if math.Abs(out[11]-0.75*gain) > 0.2 || out[12] <= 0 || out[14] != 0 {
		t.Errorf("ассист и тимкилл: %+v", out)
	}
	if out[13] >= 0 {
		t.Errorf("убивший союзника должен потерять: %v", out[13])
	}
	if math.Abs(sum(out)) > 1e-9 {
		t.Errorf("сумма по раунду %v", sum(out))
	}
	if RoundSwing(SwingRound{Restored: true, Winner: "A", Kills: r.Kills}, teams5()) != nil {
		t.Error("восстановленный раунд не считается")
	}
}

func TestMatchValue(t *testing.T) {
	if v := MatchValue(1.10, 0, true); math.Abs(v-(0.67*1.10+0.33)) > 1e-9 {
		t.Errorf("нулевой Round Swing: %v", v)
	}
	if MatchValue(1.10, 0, false) != 1.10 {
		t.Error("без данных значение — Rating 2.0")
	}
	if MatchValue(1.0, 0.02, true) <= MatchValue(1.0, -0.01, true) {
		t.Error("при одинаковом Rating выше Round Swing — выше значение")
	}
}
