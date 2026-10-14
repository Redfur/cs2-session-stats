package stats

import "math"

// Константы шкалы 0–100. Подобраны один раз по матчам компании (design.md add-player-rating):
// после этого их не меняют, иначе сдвинется вся история оценок.
// Средний Rating 2.0 компании — 1.084 (200 строк игрок-матч, октябрь 2026), он получает 60 баллов,
// постоянные игроки укладываются в 38–80, лучший матч — 98.
const (
	scoreSlope  = 3.0
	scoreCenter = 0.95 // rating, которому соответствует 50 баллов
)

// Score переводит Rating 2.0 в оценку 0–100 плавной S-образной функцией.
// Средние считаются по rating и только потом переводятся в баллы: шкала нелинейна.
func Score(rating float64) float64 {
	return 100 / (1 + math.Exp(-scoreSlope*(rating-scoreCenter)))
}

// ScorePoints — оценка, округлённая до целых баллов, как она показывается.
func ScorePoints(rating float64) int { return int(math.Round(Score(rating))) }
