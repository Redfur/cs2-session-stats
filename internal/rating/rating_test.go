package rating

import (
	"math"
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

func row(match, session int64, id uint64, c stats.Counters) MatchRow {
	return MatchRow{MatchID: match, SessionID: session, SteamID: id, Counters: c}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestFirstMatch(t *testing.T) {
	c := counters(24, 1.20)
	b := Build([]MatchRow{row(1, 1, 7, c)}, params)
	s, ok := b.OVR(7)
	if !ok || !near(s.Rating, c.Rating()) || s.Matches != 1 {
		t.Fatalf("OVR после первого матча = %+v, ожидался rating матча %v", s, c.Rating())
	}
	if _, ok := b.OVR(8); ok {
		t.Error("у игрока без матчей OVR быть не должно")
	}
	if _, ok := b.Before(7, 1); ok {
		t.Error("перед первым матчем OVR быть не должно")
	}
}

func TestFewMatchesIsMean(t *testing.T) {
	a, c := counters(24, 0.80), counters(24, 1.20)
	b := Build([]MatchRow{row(1, 1, 7, a), row(2, 1, 7, c)}, params)
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
		rows = append(rows, row(int64(i+1), 1, 7, c))
	}
	s, _ := Build(rows, params).OVR(7)
	if s.Rating <= sum.Rating()+0.01 {
		t.Errorf("OVR = %v: новые матчи должны весить больше, чем в среднем по всем (%v)", s.Rating, sum.Rating())
	}
}

func TestCalibration(t *testing.T) {
	var rows []MatchRow
	for i := 0; i < 10; i++ {
		rows = append(rows, row(int64(i+1), 1, 7, counters(24, 1)))
	}
	b := Build(rows[:9], params)
	if s, _ := b.OVR(7); !b.Calibrating(s.Matches) {
		t.Error("9 матчей — ещё калибровка")
	}
	b = Build(rows, params)
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
	b := Build(rows[:4], params)
	first, ok := b.Session(7, 1)
	if !ok || first.HasBefore {
		t.Fatalf("первая сессия игрока: %+v, ожидалось без «до»", first)
	}
	if first.Counters != rows[0].Counters.Add(rows[1].Counters) {
		t.Errorf("счётчики сессии %+v", first.Counters)
	}
	second, _ := b.Session(7, 2)
	before, _ := b.Before(7, 3)
	if !second.HasBefore || !near(second.Before.Rating, first.After.Rating) || !near(before.Rating, first.After.Rating) || before.Matches != 2 {
		t.Errorf("перед второй сессией R̄ = %+v, ожидалось %+v", second.Before, first.After)
	}

	// следующая сессия не меняет изменение за прошлую
	b = Build(rows, params)
	if again, _ := b.Session(7, 1); again != first {
		t.Errorf("изменение за первую сессию стало %+v, было %+v", again, first)
	}
	second, _ = b.Session(7, 2)
	last, _ := b.LastSession(7)
	ovr, _ := b.OVR(7)
	if last != second || !near(second.After.Rating, ovr.Rating) || second.After.Matches != 4 {
		t.Errorf("последняя сессия %+v, OVR %+v", last, ovr)
	}
	if _, ok := b.Session(8, 1); ok {
		t.Error("игрок не играл в сессии 1")
	}
}

func TestLast(t *testing.T) {
	var rows []MatchRow
	var want stats.Counters
	for i := 0; i < 25; i++ {
		c := counters(20+i, 1)
		if i >= 5 {
			want = want.Add(c)
		}
		rows = append(rows, row(int64(i+1), 1, 7, c))
	}
	b := Build(rows, params)
	if got, _ := b.Last(7, 20); got != want {
		t.Errorf("сумма последних 20 = %+v, ожидалось %+v", got, want)
	}
	if got, _ := Build(rows[:3], params).Last(7, 20); got.Rounds != 20+21+22 {
		t.Errorf("при 3 матчах берутся все: %d раундов", got.Rounds)
	}
}
