package stats

import (
	"testing"

	"cs2stats/internal/parser"
)

func duelMap(m parser.Match) map[[2]uint64]int {
	got := map[[2]uint64]int{}
	for _, d := range Duels(m) {
		got[[2]uint64{d.Killer, d.Victim}] = d.Kills
	}
	return got
}

func TestDuels(t *testing.T) {
	trade := kill(11, b2, a1) // размен: b2 убил a1
	rounds := []parser.Round{
		{Kills: []parser.Kill{kill(10, a1, b1), trade}},
		{Kills: []parser.Kill{
			kill(5, a1, a2), // тимкилл
			kill(6, b1, b1), // суицид
			kill(7, 0, b2),  // смерть от мира
			kill(8, a1, b1),
		}},
	}
	m := parser.Match{Players: players(), Rounds: rounds}
	got := duelMap(m)

	// 2×2 соперника → 8 направленных пар, союзников нет
	if len(got) != 8 {
		t.Fatalf("пар %d, ожидалось 8: %v", len(got), got)
	}
	want := map[[2]uint64]int{{a1, b1}: 2, {b2, a1}: 1, {b1, a1}: 0, {a2, b2}: 0}
	for k, v := range want {
		if n, ok := got[k]; !ok || n != v {
			t.Errorf("%v = %d (есть: %v), ожидалось %d", k, n, ok, v)
		}
	}
	if _, ok := got[[2]uint64{a1, a2}]; ok {
		t.Error("пара союзников в дуэлях")
	}

	// сумма дуэлей игрока совпадает с его K
	sum := map[uint64]int{}
	for k, v := range got {
		sum[k[0]] += v
	}
	for _, p := range Compute(m) {
		if sum[p.SteamID] != p.Kills {
			t.Errorf("%s: дуэли %d, K %d", p.Name, sum[p.SteamID], p.Kills)
		}
	}
}

func TestDuelsDuplicatePlayer(t *testing.T) {
	// игрок, переподключившийся в матче, встречается в Players дважды — пары не дублируются
	m := parser.Match{Players: append(players(), parser.Player{SteamID: a1, Name: "a1", Team: parser.TeamA})}
	if n := len(Duels(m)); n != 8 {
		t.Errorf("пар %d, ожидалось 8", n)
	}
}
