package stats

import "math"

// Константы шкалы 0–100. Подобраны один раз по матчам компании (design.md add-player-rating):
// после этого их не меняют, иначе сдвинется вся история оценок.
// Шкала переводит значение матча (Rating 2.0 и Round Swing, internal/rating). Среднее значение
// компании — 1.056 (180 строк игрок-матч, 18 матчей, октябрь 2026), оно получает 60 баллов,
// постоянные игроки укладываются примерно в 37–81.
const (
	scoreSlope  = 3.0
	scoreCenter = 0.921 // значение, которому соответствует 50 баллов
)

// Score переводит значение матча в оценку 0–100 плавной S-образной функцией.
// Средние считаются по rating и только потом переводятся в баллы: шкала нелинейна.
func Score(rating float64) float64 {
	return 100 / (1 + math.Exp(-scoreSlope*(rating-scoreCenter)))
}

// ScorePoints — оценка, округлённая до целых баллов, как она показывается.
func ScorePoints(rating float64) int { return int(math.Round(Score(rating))) }
