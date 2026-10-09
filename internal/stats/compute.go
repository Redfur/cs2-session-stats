package stats

import (
	"sort"
	"time"

	"cs2stats/internal/parser"
)

// TradeWindow — смерть игрока считается разменянной, если союзник убил его
// убийцу в течение этого времени.
const TradeWindow = 5 * time.Second

// Compute считает статистику каждого игрока матча.
func Compute(m parser.Match) []PlayerStats {
	team := make(map[uint64]parser.Team, len(m.Players))
	acc := make(map[uint64]*PlayerStats, len(m.Players))
	for _, p := range m.Players {
		team[p.SteamID] = p.Team
		acc[p.SteamID] = &PlayerStats{
			SteamID:  p.SteamID,
			Name:     p.Name,
			Team:     string(p.Team),
			Result:   result(p.Team, m.ScoreA, m.ScoreB),
			Counters: Counters{Rounds: len(m.Rounds)},
		}
	}
	isEnemyKill := func(k parser.Kill) bool {
		kt, ok1 := team[k.Killer]
		vt, ok2 := team[k.Victim]
		return ok1 && ok2 && k.Killer != k.Victim && kt != vt
	}

	for _, r := range m.Rounds {
		kills := append([]parser.Kill(nil), r.Kills...)
		sort.SliceStable(kills, func(i, j int) bool { return kills[i].Time < kills[j].Time })

		roundKills := map[uint64]int{}
		contributed := map[uint64]bool{} // убийство, ассист или размен
		dead := map[uint64]bool{}
		openingDone := false

		for i, k := range kills {
			if p := acc[k.Victim]; p != nil {
				p.Deaths++ // смерть засчитывается при любой причине, включая тимкилл и суицид
				dead[k.Victim] = true
			}
			if !isEnemyKill(k) {
				continue
			}
			killer := acc[k.Killer]
			killer.Kills++
			if k.Headshot {
				killer.HSKills++
			}
			roundKills[k.Killer]++
			contributed[k.Killer] = true

			if a := acc[k.Assister]; a != nil && team[k.Assister] != team[k.Victim] && k.Assister != k.Killer {
				a.Assists++
				contributed[k.Assister] = true
			}
			if !openingDone {
				openingDone = true
				killer.OpeningKills++
				acc[k.Victim].OpeningDeaths++
			}
			// размен: союзник жертвы убил её убийцу в пределах окна
			for _, later := range kills[i+1:] {
				if later.Time-k.Time > TradeWindow {
					break
				}
				if later.Victim == k.Killer && isEnemyKill(later) && team[later.Killer] == team[k.Victim] {
					contributed[k.Victim] = true
					break
				}
			}
		}

		for _, d := range r.Damages {
			if p := acc[d.Attacker]; p != nil && d.Attacker != d.Victim {
				if vt, ok := team[d.Victim]; ok && vt != team[d.Attacker] {
					p.Damage += d.Amount
				}
			}
		}

		for id, p := range acc {
			if contributed[id] || !dead[id] {
				p.KASTRounds++
			}
			switch n := roundKills[id]; {
			case n == 1:
				p.K1++
			case n == 2:
				p.K2++
			case n == 3:
				p.K3++
			case n == 4:
				p.K4++
			case n >= 5:
				p.K5++
			}
		}
	}

	out := make([]PlayerStats, 0, len(acc))
	seen := make(map[uint64]bool, len(acc))
	for _, p := range m.Players { // порядок как в матче, без повторов по SteamID
		if !seen[p.SteamID] {
			seen[p.SteamID] = true
			out = append(out, *acc[p.SteamID])
		}
	}
	return out
}

func result(t parser.Team, scoreA, scoreB int) Result {
	own, other := scoreA, scoreB
	if t == parser.TeamB {
		own, other = scoreB, scoreA
	}
	switch {
	case own > other:
		return Win
	case own < other:
		return Loss
	default:
		return Draw
	}
}
