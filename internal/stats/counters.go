// Package stats считает статистику игроков по событиям матча.
package stats

// Counters — сырые счётчики игрока. Хранятся в БД как есть, а производные
// показатели (ADR, KAST%, rating) считаются из них, поэтому формулы можно
// менять без повторного парсинга демок, а суммы по нескольким матчам дают
// корректные агрегаты за сессию.
type Counters struct {
	Rounds        int `json:"rounds"`
	Kills         int `json:"kills"`
	Deaths        int `json:"deaths"`
	Assists       int `json:"assists"`
	HSKills       int `json:"hsKills"`
	Damage        int `json:"damage"`
	KASTRounds    int `json:"kastRounds"`
	K1            int `json:"k1"`
	K2            int `json:"k2"`
	K3            int `json:"k3"`
	K4            int `json:"k4"`
	K5            int `json:"k5"`
	OpeningKills  int `json:"openingKills"`
	OpeningDeaths int `json:"openingDeaths"`
}

// Add возвращает сумму счётчиков.
func (c Counters) Add(o Counters) Counters {
	return Counters{
		Rounds:        c.Rounds + o.Rounds,
		Kills:         c.Kills + o.Kills,
		Deaths:        c.Deaths + o.Deaths,
		Assists:       c.Assists + o.Assists,
		HSKills:       c.HSKills + o.HSKills,
		Damage:        c.Damage + o.Damage,
		KASTRounds:    c.KASTRounds + o.KASTRounds,
		K1:            c.K1 + o.K1,
		K2:            c.K2 + o.K2,
		K3:            c.K3 + o.K3,
		K4:            c.K4 + o.K4,
		K5:            c.K5 + o.K5,
		OpeningKills:  c.OpeningKills + o.OpeningKills,
		OpeningDeaths: c.OpeningDeaths + o.OpeningDeaths,
	}
}

// KD — отношение убийств к смертям; без смертей равно числу убийств.
func (c Counters) KD() float64 {
	if c.Deaths == 0 {
		return float64(c.Kills)
	}
	return float64(c.Kills) / float64(c.Deaths)
}

func (c Counters) ADR() float64 { return ratio(c.Damage, c.Rounds) }

func (c Counters) HSPercent() float64 { return 100 * ratio(c.HSKills, c.Kills) }

func (c Counters) KASTPercent() float64 { return 100 * ratio(c.KASTRounds, c.Rounds) }

// Константы HLTV Rating 1.0 — средние значения по профессиональным матчам.
const (
	avgKPR             = 0.679
	avgSPR             = 0.317
	avgRMK             = 1.277
	survivalWeight     = 0.7
	ratingNormalizator = 2.7
)

// Rating считает HLTV Rating 1.0. Все слагаемые — величины «на раунд»,
// поэтому рейтинг по суммарным счётчикам нескольких матчей равен среднему
// рейтингу матчей, взвешенному по числу раундов.
func (c Counters) Rating() float64 {
	if c.Rounds == 0 {
		return 0
	}
	r := float64(c.Rounds)
	kill := float64(c.Kills) / r / avgKPR
	survival := float64(c.Rounds-c.Deaths) / r / avgSPR
	multi := float64(c.K1+4*c.K2+9*c.K3+16*c.K4+25*c.K5) / r / avgRMK
	return (kill + survivalWeight*survival + multi) / ratingNormalizator
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Result — исход матча для игрока.
type Result string

const (
	Win  Result = "win"
	Loss Result = "loss"
	Draw Result = "draw"
)

// PlayerStats — статистика игрока за один матч.
type PlayerStats struct {
	SteamID uint64
	Name    string
	Team    string // "A" или "B"
	Result  Result
	Counters
}
