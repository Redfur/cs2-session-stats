package rating

import "math"

// Модель шанса выиграть раунд (design improve-ovr, решение 1). Константы подобраны под опубликованные
// примеры HLTV Rating 3.0: двойное входное убийство за T — 48% → 89%, открывающее убийство — около +20 п.п.,
// против полного эко — около 96%. По матчам компании они не обучаются.
const (
	winBase    = -0.08 // logit(0.48): шанс T при равном раскладе и закупке
	winAlive   = 8.0   // вес доли перевеса (nT − nCT)/(nT + nCT)
	winPlant   = 1.0   // бомба установлена (допущение: 48% → 71% при равном раскладе)
	winEquip   = 1.8   // вес ln(снаряжение T / снаряжение CT)
	equipClamp = 2.5   // предел |ln| соотношения снаряжения
)

// WinProbT — шанс стороны T выиграть раунд при раскладе живых, установленной бомбе и снаряжении
// команд. Неизвестное снаряжение (0 у любой стороны) считается равным.
func WinProbT(aliveT, aliveCT int, planted bool, equipT, equipCT int) float64 {
	switch {
	case aliveCT <= 0:
		return 1
	case aliveT <= 0 && !planted:
		return 0
	}
	x := winBase + winAlive*float64(aliveT-aliveCT)/float64(aliveT+aliveCT)
	if planted {
		x += winPlant
	}
	if equipT > 0 && equipCT > 0 {
		x += winEquip * math.Max(-equipClamp, math.Min(equipClamp, math.Log(float64(equipT)/float64(equipCT))))
	}
	return 1 / (1 + math.Exp(-x))
}

// Доля заслуги убийцы, если у убийства есть ассистент (в том числе флешкой); остальное — ассистенту.
const killerShare = 0.75

// maxAlive — больше пяти игроков в раунде одной команды не бывает: замены не играют одновременно.
const maxAlive = 5

// SwingKill — убийство раунда для Round Swing. Время — от начала демки, в миллисекундах.
type SwingKill struct {
	TimeMs                   int64
	Killer, Victim, Assister uint64
}

// SwingRound — раунд матча: победитель и сторона команды A ("A"/"B", "CT"/"T"), снаряжение команд
// на конец закупки, время установки бомбы (0 — не установили) и убийства в порядке времени.
type SwingRound struct {
	Winner, SideA  string
	EquipA, EquipB int
	PlantMs        int64
	Restored       bool
	Kills          []SwingKill
}

// SwingMatch — данные Round Swing одного матча.
type SwingMatch struct {
	Rounds []SwingRound
}

// RoundSwing распределяет изменения шанса команды выиграть раунд между игроками (design, решение 2).
// teams — команда каждого игрока матча ("A"/"B"). Сумма по раунду равна нулю; восстановленный
// раунд пропускается.
func RoundSwing(r SwingRound, teams map[uint64]string) map[uint64]float64 {
	if r.Restored {
		return nil
	}
	alive := map[string][]uint64{}
	for id, t := range teams {
		alive[t] = append(alive[t], id)
	}
	count := func(t string) int { return min(len(alive[t]), maxAlive) }
	planted := false
	// probA — шанс команды A выиграть раунд в текущем раскладе
	probA := func() float64 {
		if r.SideA == "T" {
			return WinProbT(count("A"), count("B"), planted, r.EquipA, r.EquipB)
		}
		return 1 - WinProbT(count("B"), count("A"), planted, r.EquipB, r.EquipA)
	}
	other := map[string]string{"A": "B", "B": "A"}
	out := map[uint64]float64{}
	// shareAlive делит value поровну между живыми игроками команды, а если живых нет — между всеми
	shareAlive := func(t string, value float64) {
		ids := alive[t]
		if len(ids) == 0 {
			for id, tt := range teams {
				if tt == t {
					ids = append(ids, id)
				}
			}
		}
		for _, id := range ids {
			out[id] += value / float64(len(ids))
		}
	}
	for _, k := range r.Kills {
		victimTeam, ok := teams[k.Victim]
		if !ok || !contains(alive[victimTeam], k.Victim) {
			continue
		}
		planted = r.PlantMs > 0 && k.TimeMs >= r.PlantMs
		before := probA()
		alive[victimTeam] = remove(alive[victimTeam], k.Victim)
		after := probA()
		// потеря команды жертвы — величина ≤ 0
		loss := after - before
		if victimTeam == "B" {
			loss = before - after
		}
		killerTeam, known := teams[k.Killer]
		switch {
		case known && k.Killer != k.Victim && killerTeam != victimTeam:
			gain := -loss
			if at, ok := teams[k.Assister]; ok && k.Assister != k.Killer && at == killerTeam {
				out[k.Killer] += killerShare * gain
				out[k.Assister] += (1 - killerShare) * gain
			} else {
				out[k.Killer] += gain
			}
			out[k.Victim] += loss
		case known && k.Killer != k.Victim:
			// убийство союзника: потеря команды — на убийце, выигрыш соперников — их живым
			out[k.Killer] += loss
			shareAlive(other[victimTeam], -loss)
		default:
			// смерть от мира или самоубийство
			out[k.Victim] += loss
			shareAlive(other[victimTeam], -loss)
		}
	}
	// остаток в конце раунда: победителям — до 1, проигравшим — до 0
	planted = r.PlantMs > 0
	winner, loser := r.Winner, other[r.Winner]
	if loser == "" {
		return out
	}
	pWin := probA()
	if winner == "B" {
		pWin = 1 - pWin
	}
	rest := 1 - pWin
	var positive float64
	for id, v := range out {
		if teams[id] == winner && v > 0 {
			positive += v
		}
	}
	if positive > 0 {
		for id, v := range out {
			if teams[id] == winner && v > 0 {
				out[id] += rest * v / positive
			}
		}
	} else {
		shareAlive(winner, rest)
	}
	// сохранившие оружие наказываются как проигравшие клатч
	shareAlive(loser, -rest)
	return out
}

// MatchSwing — Round Swing игроков за матч (сумма по раундам) и число учтённых раундов.
func MatchSwing(m SwingMatch, teams map[uint64]string) (map[uint64]float64, int) {
	sum := map[uint64]float64{}
	rounds := 0
	for _, r := range m.Rounds {
		if r.Restored {
			continue
		}
		rounds++
		for id, v := range RoundSwing(r, teams) {
			sum[id] += v
		}
	}
	return sum, rounds
}

// Значение матча (design, решение 3): Rating 2.0 и Round Swing за раунд с весом Round Swing в Rating 3.0.
const (
	swingWeight = 0.33
	// ±13% шанса за раунд → ±1.0 к значению: разброс Round Swing в играх компании (около ±4% за раунд
	// у постоянных игроков, втрое больше, чем на про-сцене) приведён к разбросу Rating 2.0, чтобы
	// вес Round Swing действительно был около трети (design improve-ovr, решение 3)
	swingScale = 0.13
)

// MatchValue — значение матча для OVR и оценки. Без данных Round Swing — Rating 2.0.
func MatchValue(rating20, swingPerRound float64, hasSwing bool) float64 {
	if !hasSwing {
		return rating20
	}
	return (1-swingWeight)*rating20 + swingWeight*(1+swingPerRound/swingScale)
}

func contains(ids []uint64, id uint64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func remove(ids []uint64, id uint64) []uint64 {
	out := ids[:0:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}
