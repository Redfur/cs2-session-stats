package rating

import (
	"math"
	"strconv"
	"testing"

	"cs2stats/internal/stats"
)

var params = Params{Alpha: DefaultAlpha, Calibration: 10}

// counters возвращает счётчики матча с заданным числом раундов и rating около target:
// rating линеен по урону, поэтому нужный уровень набирается уроном.
func counters(rounds int, target float64) stats.Counters {
	c := stats.Counters{Rounds: rounds, Kills: rounds * 7 / 10, Deaths: rounds * 7 / 10, Assists: rounds / 8, KASTRounds: rounds * 7 / 10}
	c.Damage = int(math.Round((target - c.Rating()) / 0.0032 * float64(rounds)))
	return c
}

// mid и sid — id матча и сессии по номеру; rating сравнивает id только на равенство.
func mid(n int) string { return "m_" + strconv.Itoa(n) }
func sid(n int) string { return "s_" + strconv.Itoa(n) }

func row(match, session int, id uint64, c stats.Counters) MatchRow {
	return MatchRow{MatchID: mid(match), SessionID: sid(session), SteamID: id, Counters: c}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestFirstMatch(t *testing.T) {
	c := counters(24, 1.20)
	b := Build([]MatchRow{row(1, 1, 7, c)}, nil, params)
	s, ok := b.OVR(7)
	if !ok || !near(s.Rating, c.Rating()) || s.Matches != 1 {
		t.Fatalf("OVR после первого матча = %+v, ожидался rating матча %v", s, c.Rating())
	}
	if _, ok := b.OVR(8); ok {
		t.Error("у игрока без матчей OVR быть не должно")
	}
	if _, ok := b.Before(7, mid(1)); ok {
		t.Error("перед первым матчем OVR быть не должно")
	}
}

func TestFewMatchesIsMean(t *testing.T) {
	a, c := counters(24, 0.80), counters(24, 1.20)
	b := Build([]MatchRow{row(1, 1, 7, a), row(2, 1, 7, c)}, nil, params)
	s, _ := b.OVR(7)
	if !near(s.Rating, a.Add(c).Rating()) || math.Abs(s.Rating-1.00) > 0.01 {
		t.Errorf("OVR = %v, ожидался rating суммы счётчиков %v (≈ 1.00)", s.Rating, a.Add(c).Rating())
	}
}

func TestRecentMatchesWeighMore(t *testing.T) {
	var rows []MatchRow
	var sum stats.Counters
	for i := 0; i < 45; i++ {
		c := counters(24, 1.00)
		if i >= 40 {
			c = counters(24, 1.30)
		}
		sum = sum.Add(c)
		rows = append(rows, row(i+1, 1, 7, c))
	}
	s, _ := Build(rows, nil, params).OVR(7)
	if s.Rating <= sum.Rating()+0.01 {
		t.Errorf("OVR = %v: новые матчи должны весить больше, чем в среднем по всем (%v)", s.Rating, sum.Rating())
	}
}

func TestCalibration(t *testing.T) {
	var rows []MatchRow
	for i := 0; i < 10; i++ {
		rows = append(rows, row(i+1, 1, 7, counters(24, 1)))
	}
	b := Build(rows[:9], nil, params)
	if s, _ := b.OVR(7); !b.Calibrating(s.Matches) {
		t.Error("9 матчей — ещё калибровка")
	}
	b = Build(rows, nil, params)
	if s, _ := b.OVR(7); b.Calibrating(s.Matches) {
		t.Error("10 матчей — калибровка закончена")
	}
}

func TestSessionChange(t *testing.T) {
	rows := []MatchRow{
		row(1, 1, 7, counters(24, 0.90)),
		row(2, 1, 7, counters(24, 1.10)),
		row(3, 2, 7, counters(24, 1.40)), row(3, 2, 8, counters(24, 1.00)),
		row(4, 2, 7, counters(24, 1.20)),
	}
	b := Build(rows[:4], nil, params)
	first, ok := b.Session(7, sid(1))
	if !ok || first.HasBefore {
		t.Fatalf("первая сессия игрока: %+v, ожидалось без «до»", first)
	}
	if want := (rows[0].Rating() + rows[1].Rating()) / 2; !near(first.Value, want) {
		t.Errorf("значение сессии %v, ожидалось среднее матчей %v", first.Value, want)
	}
	second, _ := b.Session(7, sid(2))
	before, _ := b.Before(7, mid(3))
	if !second.HasBefore || !near(second.Before.Rating, first.After.Rating) || !near(before.Rating, first.After.Rating) || before.Matches != 2 {
		t.Errorf("перед второй сессией R̄ = %+v, ожидалось %+v", second.Before, first.After)
	}

	// следующая сессия не меняет изменение за прошлую
	b = Build(rows, nil, params)
	if again, _ := b.Session(7, sid(1)); again != first {
		t.Errorf("изменение за первую сессию стало %+v, было %+v", again, first)
	}
	second, _ = b.Session(7, sid(2))
	last, _ := b.LastSession(7)
	ovr, _ := b.OVR(7)
	if last != second || !near(second.After.Rating, ovr.Rating) || second.After.Matches != 4 {
		t.Errorf("последняя сессия %+v, OVR %+v", last, ovr)
	}
	if _, ok := b.Session(8, sid(1)); ok {
		t.Error("игрок не играл в сессии 1")
	}
}

func TestLast(t *testing.T) {
	var rows []MatchRow
	var sum, rounds float64
	for i := 0; i < 25; i++ {
		c := counters(20+i, 0.8+0.02*float64(i))
		if i >= 5 {
			sum += c.Rating() * float64(c.Rounds)
			rounds += float64(c.Rounds)
		}
		rows = append(rows, row(i+1, 1, 7, c))
	}
	b := Build(rows, nil, params)
	if got, _ := b.Last(7, 20); !near(got, sum/rounds) {
		t.Errorf("среднее последних 20 = %v, ожидалось взвешенное по раундам %v", got, sum/rounds)
	}
	if _, ok := Build(nil, nil, params).Last(7, 20); ok {
		t.Error("без матчей значения нет")
	}
}

func TestBuildWithSwing(t *testing.T) {
	// два игрока с одинаковыми счётчиками; в единственном раунде 11 убивает 1 и команда B побеждает
	c := counters(1, 1.0)
	rows := []MatchRow{
		{MatchID: mid(1), SessionID: sid(1), SteamID: 1, Team: "A", Counters: c},
		{MatchID: mid(1), SessionID: sid(1), SteamID: 11, Team: "B", Counters: c},
	}
	swings := map[string]SwingMatch{mid(1): {Rounds: []SwingRound{{Winner: "B", SideA: "CT", Kills: []SwingKill{{TimeMs: 1, Killer: 11, Victim: 1}}}}}}
	b := Build(rows, swings, params)
	won, _ := b.Match(11, mid(1))
	lost, _ := b.Match(1, mid(1))
	if !won.HasSwing || won.Swing <= 0 || lost.Swing >= 0 || won.Value <= lost.Value || !near(won.Swing, -lost.Swing) {
		t.Fatalf("с Round Swing: победитель %+v, проигравший %+v", won, lost)
	}
	if !near(won.Value, MatchValue(c.Rating(), won.Swing, true)) {
		t.Errorf("значение матча %v", won.Value)
	}
	// без данных Round Swing значение — Rating 2.0
	plain, _ := Build(rows, nil, params).Match(11, mid(1))
	if plain.HasSwing || !near(plain.Value, c.Rating()) {
		t.Errorf("без данных: %+v", plain)
	}
}
