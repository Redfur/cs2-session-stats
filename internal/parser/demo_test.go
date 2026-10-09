package parser

import (
	"os"
	"strconv"
	"testing"
)

// TestParseRealDemo проверяет парсер на настоящей демке. Путь задаётся через
// CS2STATS_TEST_DEMO (демки в репозиторий не коммитим), без него тест пропускается.
// Ожидаемые значения (CS2STATS_TEST_ROUNDS, CS2STATS_TEST_SCORE="13:9") берутся с табло платформы.
func TestParseRealDemo(t *testing.T) {
	path := os.Getenv("CS2STATS_TEST_DEMO")
	if path == "" {
		t.Skip("CS2STATS_TEST_DEMO не задан")
	}
	m, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("карта=%s раундов=%d счёт=%d:%d игроков=%d", m.Map, len(m.Rounds), m.ScoreA, m.ScoreB, len(m.Players))

	if m.Map == "" {
		t.Error("карта не определена")
	}
	if m.ScoreA+m.ScoreB != len(m.Rounds) {
		t.Errorf("сумма счёта %d != числу раундов %d", m.ScoreA+m.ScoreB, len(m.Rounds))
	}
	for _, p := range m.Players {
		if p.SteamID == 0 || (p.Team != TeamA && p.Team != TeamB) {
			t.Errorf("некорректный игрок: %+v", p)
		}
	}
	if want := os.Getenv("CS2STATS_TEST_ROUNDS"); want != "" {
		if got := len(m.Rounds); want != strconv.Itoa(got) {
			t.Errorf("раундов %d, ожидалось %s", got, want)
		}
	}
	if want := os.Getenv("CS2STATS_TEST_SCORE"); want != "" {
		if got := strconv.Itoa(m.ScoreA) + ":" + strconv.Itoa(m.ScoreB); got != want {
			t.Errorf("счёт %s, ожидалось %s", got, want)
		}
	}
}
