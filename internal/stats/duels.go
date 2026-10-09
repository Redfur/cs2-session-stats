package stats

import (
	"sort"

	"cs2stats/internal/parser"
)

// Duel — сколько раз Killer убил Victim за матч. Смерти Killer от Victim —
// это обратная пара, отдельно не хранятся.
type Duel struct {
	Killer uint64
	Victim uint64
	Kills  int
}

// Duels считает личные убийства для каждой направленной пары соперников матча,
// включая пары без убийств. Учитываются те же убийства, что и в K.
func Duels(m parser.Match) []Duel {
	team := teams(m)
	ids := make([]uint64, 0, len(team))
	for id := range team {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	type pair struct{ killer, victim uint64 }
	kills := map[pair]int{}
	for _, r := range m.Rounds {
		for _, k := range r.Kills {
			if enemyKill(team, k) {
				kills[pair{k.Killer, k.Victim}]++
			}
		}
	}

	var out []Duel
	for _, killer := range ids {
		for _, victim := range ids {
			if team[killer] != team[victim] {
				out = append(out, Duel{Killer: killer, Victim: victim, Kills: kills[pair{killer, victim}]})
			}
		}
	}
	return out
}
