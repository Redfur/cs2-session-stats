// Package rating считает общий уровень игроков (OVR) по упорядоченному набору матчей.
// Это чистая функция от сохранённых счётчиков: ничего не хранится, история пересчитывается
// на каждый запрос, поэтому перестановка, удаление и пересчёт матча сразу меняют OVR.
package rating

import (
	"math"

	"cs2stats/internal/stats"
)

// DefaultAlpha — вес матча из 24 раундов в EMA, память примерно как у окна в 20 матчей.
// Подбирается проверкой на прошлых матчах (roadmap 1.3).
const DefaultAlpha = 0.1

// fullMatchRounds — длина матча, вес которого в EMA равен Alpha.
const fullMatchRounds = 24

// Params — параметры расчёта.
type Params struct {
	Alpha       float64
	Calibration int // сколько матчей нужно, чтобы OVR перестал быть калибровочным
}

// MatchRow — один игрок в одном матче. Строки идут в порядке игры, строки одного матча — подряд.
type MatchRow struct {
	MatchID   int64
	SessionID int64
	SteamID   uint64
	Team      string // для будущей поправки на силу соперников (roadmap 1.3)
	stats.Counters
}

// Step — состояние игрока до и после одного его матча.
type Step struct {
	MatchID   int64
	SessionID int64
	Counters  stats.Counters
	Before    float64 // R̄ перед матчем; не определён, если матч первый (Index == 0)
	After     float64 // R̄ после матча
	Index     int     // сколько матчей у игрока было до этого
}

// State — R̄ игрока (рейтинг до шкалы) и число матчей, по которым он получен.
type State struct {
	Rating  float64
	Matches int
}

// Score — R̄ по шкале 0–100, округлённый до целых.
func (s State) Score() int { return stats.ScorePoints(s.Rating) }

// Book — история всех игроков.
type Book struct {
	params  Params
	players map[uint64][]Step
}

// Build проходит матчи в порядке игры и обновляет R̄ каждого игрока:
//
//	w  = max(R / раунды игрока с этим матчем, 1 − (1 − α)^(R/24))
//	R̄ ← R̄ + w·(rating матча − R̄), первый матч: R̄ = rating матча
//
// Первая часть max даёт обычное среднее по сумме счётчиков, пока матчей мало,
// вторая — экспоненциальное затухание, при котором новые матчи весят больше.
func Build(rows []MatchRow, p Params) *Book {
	b := &Book{params: p, players: map[uint64][]Step{}}
	rounds := map[uint64]int{}
	for start := 0; start < len(rows); {
		end := start + 1
		for end < len(rows) && rows[end].MatchID == rows[start].MatchID {
			end++
		}
		// rows[start:end] — состав одного матча: союзники и соперники для будущей поправки γ
		for _, r := range rows[start:end] {
			if r.Rounds == 0 {
				continue
			}
			steps := b.players[r.SteamID]
			rounds[r.SteamID] += r.Rounds
			rm := r.Rating()
			st := Step{MatchID: r.MatchID, SessionID: r.SessionID, Counters: r.Counters, Index: len(steps), After: rm}
			if len(steps) > 0 {
				st.Before = steps[len(steps)-1].After
				w := math.Max(float64(r.Rounds)/float64(rounds[r.SteamID]),
					1-math.Pow(1-p.Alpha, float64(r.Rounds)/fullMatchRounds))
				st.After = st.Before + w*(rm-st.Before)
			}
			b.players[r.SteamID] = append(steps, st)
		}
		start = end
	}
	return b
}

// Calibrating — меньше ли n матчей порога калибровки.
func (b *Book) Calibrating(n int) bool { return n < b.params.Calibration }

// OVR — текущий R̄ игрока; ok = false, если у него нет матчей.
func (b *Book) OVR(steamID uint64) (State, bool) {
	steps := b.players[steamID]
	if len(steps) == 0 {
		return State{}, false
	}
	return State{Rating: steps[len(steps)-1].After, Matches: len(steps)}, true
}

// Before — R̄ игрока перед матчем; ok = false, если матч у него первый или игрок в нём не учтён.
func (b *Book) Before(steamID uint64, matchID int64) (State, bool) {
	for _, st := range b.players[steamID] {
		if st.MatchID == matchID {
			if st.Index == 0 {
				return State{}, false
			}
			return State{Rating: st.Before, Matches: st.Index}, true
		}
	}
	return State{}, false
}

// SessionChange — R̄ игрока перед первым его матчем в сессии и после последнего.
type SessionChange struct {
	SessionID int64
	Before    State
	HasBefore bool // false — сессия у игрока первая
	After     State
	Counters  stats.Counters // сумма счётчиков игрока за сессию
}

// Session — изменение R̄ игрока за сессию; ok = false, если он в ней не играл.
func (b *Book) Session(steamID uint64, sessionID int64) (SessionChange, bool) {
	ch := SessionChange{SessionID: sessionID}
	found := false
	for _, st := range b.players[steamID] {
		if st.SessionID != sessionID {
			continue
		}
		if !found {
			found = true
			ch.HasBefore = st.Index > 0
			ch.Before = State{Rating: st.Before, Matches: st.Index}
		}
		ch.After = State{Rating: st.After, Matches: st.Index + 1}
		ch.Counters = ch.Counters.Add(st.Counters)
	}
	return ch, found
}

// LastSession — изменение R̄ за последнюю сессию игрока; ok = false, если матчей нет.
func (b *Book) LastSession(steamID uint64) (SessionChange, bool) {
	steps := b.players[steamID]
	if len(steps) == 0 {
		return SessionChange{}, false
	}
	return b.Session(steamID, steps[len(steps)-1].SessionID)
}

// Last — сумма счётчиков последних n матчей игрока (все, если матчей меньше).
func (b *Book) Last(steamID uint64, n int) (stats.Counters, bool) {
	steps := b.players[steamID]
	if len(steps) == 0 {
		return stats.Counters{}, false
	}
	var sum stats.Counters
	for _, st := range steps[max(0, len(steps)-n):] {
		sum = sum.Add(st.Counters)
	}
	return sum, true
}
