package api

import (
	"math"
	"net/http"
	"sort"
	"strconv"

	"cs2stats/internal/parser"
	"cs2stats/internal/stats"
	"cs2stats/internal/store"
)

// Причины, по которым у матча нет значения метрики.
const (
	missingOld      = "old"       // матч обработан версией без расширенной статистики
	missingNoDamage = "no_damage" // в демке нет событий урона
	missingNoFlash  = "no_flash"  // в демке нет событий ослепления
)

type missingMatch struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"sessionId"`
	Ordinal   int    `json:"ordinal"`
	Map       string `json:"map"`
	Reason    string `json:"reason"`
}

// metric — значение с покрытием: по скольким матчам выборки оно посчитано и каких матчей нет.
// Value = null, если нет ни одного покрытого матча или у доли нет знаменателя.
type metric struct {
	Value   *float64       `json:"value"`
	Covered int            `json:"covered"`
	Total   int            `json:"total"`
	Missing []missingMatch `json:"missing"`
}

// need — что нужно матчу, чтобы метрика была посчитана.
type need int

const (
	needExt    need = iota // расширенные данные
	needDamage             // и события урона
	needFlash              // и события ослепления
)

func missingReason(r store.ExtRow, n need) string {
	switch {
	case !r.Covered:
		return missingOld
	case n == needDamage && !r.HasDamage:
		return missingNoDamage
	case n == needFlash && !r.HasFlash:
		return missingNoFlash
	}
	return ""
}

// split делит строки на покрытые метрикой и список непокрытых матчей.
func split(rows []store.ExtRow, n need) (covered []store.ExtRow, missing []missingMatch) {
	missing = []missingMatch{}
	for _, r := range rows {
		if reason := missingReason(r, n); reason != "" {
			missing = append(missing, missingMatch{ID: r.MatchID, SessionID: r.SessionID, Ordinal: r.Ordinal, Map: r.Map, Reason: reason})
			continue
		}
		covered = append(covered, r)
	}
	return covered, missing
}

func ptr(x float64) *float64 { return &x }

func ratio(num, den float64) *float64 {
	if den == 0 {
		return nil
	}
	return ptr(round2(num / den))
}

// perMatch — среднее значения за покрытый матч.
func perMatch(rows []store.ExtRow, n need, value func(stats.ExtCounters) int) metric {
	covered, missing := split(rows, n)
	var sum int
	for _, r := range covered {
		sum += value(r.Ext)
	}
	return metric{Value: ratio(float64(sum), float64(len(covered))), Covered: len(covered), Total: len(rows), Missing: missing}
}

func coverage(rows []store.ExtRow, n need) metric {
	covered, missing := split(rows, n)
	return metric{Covered: len(covered), Total: len(rows), Missing: missing}
}

func sumExt(rows []store.ExtRow) (c stats.ExtCounters, kills, deaths, assists int) {
	for _, r := range rows {
		c = c.Add(r.Ext)
		kills += r.Kills
		deaths += r.Deaths
		assists += r.Assists
	}
	return
}

type matchRef struct {
	ID        int64  `json:"id"`
	SessionID int64  `json:"sessionId"`
	Ordinal   int    `json:"ordinal"`
	Error     string `json:"error,omitempty"`
}

// extHeader — общая часть ответов: статус, матчи выборки без расширенных данных,
// пересчитываемые матчи и матчи с ошибкой последнего пересчёта.
type extHeader struct {
	Status           string         `json:"status"`
	EligibleMatches  int            `json:"eligibleMatches"`
	CoveredMatches   int            `json:"coveredMatches"`
	UncoveredMatches []missingMatch `json:"uncoveredMatches"`
	Reparsing        []matchRef     `json:"reparsing"`
	Failed           []matchRef     `json:"failed"`
}

func newExtHeader(rows []store.ExtRow) extHeader {
	covered, missing := split(rows, needExt)
	h := extHeader{
		Status: duelsStatus(len(rows), len(covered)), EligibleMatches: len(rows), CoveredMatches: len(covered),
		UncoveredMatches: missing, Reparsing: []matchRef{}, Failed: []matchRef{},
	}
	for _, r := range rows {
		ref := matchRef{ID: r.MatchID, SessionID: r.SessionID, Ordinal: r.Ordinal}
		switch r.Status {
		case store.StatusPending, store.StatusParsing:
			h.Reparsing = append(h.Reparsing, ref)
		case store.StatusFailed:
			ref.Error = r.Error
			h.Failed = append(h.Failed, ref)
		}
	}
	return h
}

// playerExtRows разбирает запрос вкладки профиля. ok=false — ответ уже записан.
func (s *Server) playerExtRows(w http.ResponseWriter, r *http.Request) (steamID uint64, f store.MatchFilter, rows []store.ExtRow, ok bool) {
	steamID, err := strconv.ParseUint(r.PathValue("steamId"), 10, 64)
	if err != nil || steamID == 0 {
		writeError(w, http.StatusBadRequest, "некорректный SteamID")
		return
	}
	if f, err = parsePeriod(r.URL.Query()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	all, err := s.Store.PlayerTotals(r.Context(), store.MatchFilter{PlayerIDs: []uint64{steamID}}, 1)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if len(all) == 0 {
		writeError(w, http.StatusNotFound, "игрок не найден")
		return
	}
	if rows, err = s.Store.PlayerExtRows(r.Context(), steamID, f); err != nil {
		s.internalError(w, err)
		return
	}
	return steamID, f, rows, true
}

// ── Вкладка «Бой» ──

type clutchRow struct {
	Vs       int      `json:"vs"` // 0 — строка «Всего»
	Attempts int      `json:"attempts"`
	Wins     int      `json:"wins"`
	Losses   int      `json:"losses"`
	Draws    int      `json:"draws"`
	Unknown  int      `json:"unknown"`
	WinRate  *float64 `json:"winRate"` // wins / (wins + losses), 0..1
}

func (c *clutchRow) add(o stats.ClutchOutcome, n int) {
	c.Attempts += n
	switch o {
	case stats.ClutchWin:
		c.Wins += n
	case stats.ClutchLoss:
		c.Losses += n
	case stats.ClutchDraw:
		c.Draws += n
	default:
		c.Unknown += n
	}
}

func (c *clutchRow) finish() {
	if d := c.Wins + c.Losses; d > 0 {
		c.WinRate = ptr(round2(float64(c.Wins) / float64(d)))
	}
}

type survivalRow struct {
	Side        string   `json:"side"` // T | CT | total
	Rounds      int      `json:"rounds"`
	Survived    int      `json:"survived"`
	Share       *float64 `json:"share"`
	AvgAliveSec *float64 `json:"avgAliveSec"`
}

func newSurvivalRow(side string, rounds, survived int, aliveMs int64) survivalRow {
	r := survivalRow{Side: side, Rounds: rounds, Survived: survived, Share: ratio(float64(survived), float64(rounds))}
	if rounds > 0 {
		r.AvgAliveSec = ptr(math.Round(float64(aliveMs) / 1000 / float64(rounds)))
	}
	return r
}

type fightResponse struct {
	extHeader
	Trades struct {
		metric
		TradedDeaths int `json:"tradedDeaths"`
		Deaths       int `json:"deaths"`
		TradeKills   int `json:"tradeKills"`
		Kills        int `json:"kills"`
	} `json:"trades"`
	Damage struct {
		metric
		DealtPerRound *float64 `json:"dealtPerRound"`
		TakenPerRound *float64 `json:"takenPerRound"`
		DiffPerRound  *float64 `json:"diffPerRound"`
		Rounds        int      `json:"rounds"`
	} `json:"damage"`
	Clutches struct {
		metric
		Rows []clutchRow `json:"rows"`
		Sum  clutchRow   `json:"sum"` // строка «Всего»
	} `json:"clutches"`
	Survival struct {
		metric
		Rows []survivalRow `json:"rows"`
	} `json:"survival"`
	Assists struct {
		Flash   int `json:"flash"`
		Damage  int `json:"damage"`
		Unknown int `json:"unknown"` // ассисты матчей без расширенных данных и восстановленных раундов
		Total   int `json:"total"`
	} `json:"assists"`
}

func (s *Server) getPlayerFight(w http.ResponseWriter, r *http.Request) {
	steamID, f, rows, ok := s.playerExtRows(w, r)
	if !ok {
		return
	}
	clutches, err := s.Store.PlayerClutches(r.Context(), steamID, f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	resp := fightResponse{extHeader: newExtHeader(rows)}
	covered, _ := split(rows, needExt)
	ext, kills, deaths, _ := sumExt(covered)

	resp.Trades.metric = coverage(rows, needExt)
	resp.Trades.TradedDeaths, resp.Trades.Deaths = ext.TradedDeaths, deaths
	resp.Trades.TradeKills, resp.Trades.Kills = ext.TradeKills, kills

	resp.Damage.metric = coverage(rows, needDamage)
	dmgRows, _ := split(rows, needDamage)
	dmg, _, _, _ := sumExt(dmgRows)
	rounds := float64(dmg.Rounds)
	resp.Damage.Rounds = dmg.Rounds
	resp.Damage.DealtPerRound = ratio(float64(dmg.DamageDealt), rounds)
	resp.Damage.TakenPerRound = ratio(float64(dmg.DamageTaken), rounds)
	resp.Damage.DiffPerRound = ratio(float64(dmg.DamageDealt-dmg.DamageTaken), rounds)

	resp.Clutches.metric = coverage(rows, needExt)
	byVs := make([]clutchRow, 5)
	for i := range byVs {
		byVs[i].Vs = i + 1
	}
	for _, c := range clutches {
		if c.Vs >= 1 && c.Vs <= 5 {
			byVs[c.Vs-1].add(c.Outcome, c.Count)
			resp.Clutches.Sum.add(c.Outcome, c.Count)
		}
	}
	for i := range byVs {
		byVs[i].finish()
	}
	resp.Clutches.Sum.finish()
	resp.Clutches.Rows = byVs

	resp.Survival.metric = coverage(rows, needExt)
	resp.Survival.Rows = []survivalRow{
		newSurvivalRow("T", ext.RoundsT, ext.SurvivedT, ext.AliveMsT),
		newSurvivalRow("CT", ext.RoundsCT, ext.SurvivedCT, ext.AliveMsCT),
		newSurvivalRow("total", ext.RoundsT+ext.RoundsCT, ext.SurvivedT+ext.SurvivedCT, ext.AliveMsT+ext.AliveMsCT),
	}

	_, _, _, assists := sumExt(rows)
	resp.Assists.Flash, resp.Assists.Damage = ext.FlashAssists, ext.DamageAssists
	resp.Assists.Total = assists
	resp.Assists.Unknown = max(0, assists-ext.FlashAssists-ext.DamageAssists)
	writeJSON(w, http.StatusOK, resp)
}

// ── Вкладка «Оружие» ──

type weaponView struct {
	SteamID    string   `json:"steamId,omitempty"`
	Weapon     string   `json:"weapon"`
	Kills      int      `json:"kills"`
	HSKills    int      `json:"hsKills"`
	HSKillsPct *float64 `json:"hsKillsPct"`
	Damage     int      `json:"damage"`
	Shots      int      `json:"shots"`
	Hits       int      `json:"hits"`
	HSHits     int      `json:"hsHits"`
	HSHitsPct  *float64 `json:"hsHitsPct"`
}

func pct(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	return ptr(math.Round(100 * float64(num) / float64(den)))
}

func newWeaponView(w store.WeaponTotal) weaponView {
	return weaponView{
		Weapon: w.Weapon, Kills: w.Kills, HSKills: w.HSKills, HSKillsPct: pct(w.HSKills, w.Kills),
		Damage: w.Damage, Shots: w.Shots, Hits: w.Hits, HSHits: w.HSHits, HSHitsPct: pct(w.HSHits, w.Hits),
	}
}

type weaponsResponse struct {
	extHeader
	Weapons struct {
		metric
		Damage metric       `json:"damage"` // покрытие урона и попаданий
		Rows   []weaponView `json:"rows"`
	} `json:"weapons"`
	GrenadeKills int `json:"grenadeKills"`
	KillDetails  struct {
		metric
		Kills       int      `json:"kills"`
		Smoke       int      `json:"smoke"`
		Wallbang    int      `json:"wallbang"`
		NoScope     int      `json:"noScope"`
		Blind       int      `json:"blind"`
		AvgDistance *float64 `json:"avgDistance"` // единицы игры
	} `json:"killDetails"`
}

func (s *Server) getPlayerWeapons(w http.ResponseWriter, r *http.Request) {
	steamID, f, rows, ok := s.playerExtRows(w, r)
	if !ok {
		return
	}
	weapons, err := s.Store.PlayerWeapons(r.Context(), steamID, f)
	if err != nil {
		s.internalError(w, err)
		return
	}
	resp := weaponsResponse{extHeader: newExtHeader(rows)}
	resp.Weapons.metric = coverage(rows, needExt)
	resp.Weapons.Damage = coverage(rows, needDamage)
	resp.Weapons.Rows = []weaponView{}
	for _, wt := range weapons {
		resp.Weapons.Rows = append(resp.Weapons.Rows, newWeaponView(wt))
	}
	covered, _ := split(rows, needExt)
	ext, kills, _, _ := sumExt(covered)
	resp.GrenadeKills = ext.GrenadeKills()
	d := &resp.KillDetails
	d.metric = coverage(rows, needExt)
	d.Kills, d.Smoke, d.Wallbang, d.NoScope, d.Blind = kills, ext.SmokeKills, ext.WallbangKills, ext.NoScopeKills, ext.BlindKills
	if ext.DistanceKills > 0 {
		d.AvgDistance = ptr(math.Round(ext.DistanceSum / float64(ext.DistanceKills)))
	}
	writeJSON(w, http.StatusOK, resp)
}

// ── Вкладка «Гранаты и бомба» ──

type grenadeMap struct {
	Map     string `json:"map"`
	Matches int    `json:"matches"`
	HE      metric `json:"he"`
	Fire    metric `json:"fire"`
	Flashed metric `json:"flashed"`
	Smokes  metric `json:"smokes"`
	Kills   metric `json:"kills"` // убийства HE и огнём за период, сумма
}

// sumMetric — сумма значения по покрытым матчам.
func sumMetric(rows []store.ExtRow, n need, value func(stats.ExtCounters) int) metric {
	covered, missing := split(rows, n)
	m := metric{Covered: len(covered), Total: len(rows), Missing: missing}
	if len(covered) > 0 {
		var sum int
		for _, r := range covered {
			sum += value(r.Ext)
		}
		m.Value = ptr(float64(sum))
	}
	return m
}

func grenadeMetrics(rows []store.ExtRow) (he, fire, flashed, smokes metric) {
	he = perMatch(rows, needDamage, func(c stats.ExtCounters) int { return c.HEDamage })
	fire = perMatch(rows, needDamage, func(c stats.ExtCounters) int { return c.FireDamage })
	flashed = perMatch(rows, needFlash, func(c stats.ExtCounters) int { return c.Flashed })
	smokes = perMatch(rows, needExt, func(c stats.ExtCounters) int { return c.Smokes })
	return
}

type utilityResponse struct {
	extHeader
	Grenades struct {
		HE           metric `json:"he"`
		Fire         metric `json:"fire"`
		Flashed      metric `json:"flashed"`
		Smokes       metric `json:"smokes"`
		GrenadeKills struct {
			metric
			Count int `json:"count"` // все убийства гранатами за период
			HE    int `json:"he"`
			Fire  int `json:"fire"`
		} `json:"grenadeKills"`
	} `json:"grenades"`
	ByMap []grenadeMap `json:"byMap"`
	Bomb  struct {
		metric
		Plants         int `json:"plants"`
		PlantStarts    int `json:"plantStarts"`
		PlantsAborted  int `json:"plantsAborted"`
		Defuses        int `json:"defuses"`
		DefuseStarts   int `json:"defuseStarts"`
		DefusesAborted int `json:"defusesAborted"`
	} `json:"bomb"`
}

func (s *Server) getPlayerUtility(w http.ResponseWriter, r *http.Request) {
	_, _, rows, ok := s.playerExtRows(w, r)
	if !ok {
		return
	}
	resp := utilityResponse{extHeader: newExtHeader(rows)}
	g := &resp.Grenades
	g.HE, g.Fire, g.Flashed, g.Smokes = grenadeMetrics(rows)
	covered, _ := split(rows, needExt)
	ext, _, _, _ := sumExt(covered)
	g.GrenadeKills.metric = coverage(rows, needExt)
	g.GrenadeKills.Count, g.GrenadeKills.HE, g.GrenadeKills.Fire = ext.GrenadeKills(), ext.HEKills, ext.FireKills

	byMap := map[string][]store.ExtRow{}
	var maps []string
	for _, row := range rows {
		if byMap[row.Map] == nil {
			maps = append(maps, row.Map)
		}
		byMap[row.Map] = append(byMap[row.Map], row)
	}
	sort.SliceStable(maps, func(i, j int) bool {
		if a, b := len(byMap[maps[i]]), len(byMap[maps[j]]); a != b {
			return a > b
		}
		return maps[i] < maps[j]
	})
	resp.ByMap = []grenadeMap{}
	for _, name := range maps {
		gm := grenadeMap{Map: name, Matches: len(byMap[name])}
		gm.HE, gm.Fire, gm.Flashed, gm.Smokes = grenadeMetrics(byMap[name])
		gm.Kills = sumMetric(byMap[name], needExt, func(c stats.ExtCounters) int { return c.HEKills + c.FireKills })
		resp.ByMap = append(resp.ByMap, gm)
	}

	b := &resp.Bomb
	b.metric = coverage(rows, needExt)
	b.Plants, b.PlantStarts, b.PlantsAborted = ext.Plants, ext.PlantStarts, max(0, ext.PlantStarts-ext.Plants)
	b.Defuses, b.DefuseStarts, b.DefusesAborted = ext.Defuses, ext.DefuseStarts, max(0, ext.DefuseStarts-ext.Defuses)
	writeJSON(w, http.StatusOK, resp)
}

// ── Подробные колонки игрока: матч и сессия ──

type detailView struct {
	SteamID        string   `json:"steamId"`
	Matches        int      `json:"matches"` // матчи с расширенными данными
	TakenPerRound  *float64 `json:"takenPerRound"`
	DiffPerRound   *float64 `json:"diffPerRound"`
	TradeKills     int      `json:"tradeKills"`
	TradedDeaths   int      `json:"tradedDeaths"`
	ClutchWins     int      `json:"clutchWins"`
	ClutchAttempts int      `json:"clutchAttempts"`
	FlashAssists   int      `json:"flashAssists"`
	Flashed        *float64 `json:"flashed"`       // за карту
	HEDamage       *float64 `json:"heDamage"`      // за карту
	FireDamage     *float64 `json:"fireDamage"`    // за карту
	GrenadeDamage  *float64 `json:"grenadeDamage"` // HE + огонь за карту
	Smokes         *float64 `json:"smokes"`        // за карту
	SurvivedPct    *float64 `json:"survivedPct"`   // 0..100
}

func newDetailView(steamID uint64, rows []store.ExtRow, clutches []store.ClutchCount) detailView {
	v := detailView{SteamID: strconv.FormatUint(steamID, 10)}
	covered, _ := split(rows, needExt)
	ext, _, _, _ := sumExt(covered)
	v.Matches = len(covered)
	v.TradeKills, v.TradedDeaths, v.FlashAssists = ext.TradeKills, ext.TradedDeaths, ext.FlashAssists
	dmgRows, _ := split(rows, needDamage)
	dmg, _, _, _ := sumExt(dmgRows)
	v.TakenPerRound = ratio(float64(dmg.DamageTaken), float64(dmg.Rounds))
	v.DiffPerRound = ratio(float64(dmg.DamageDealt-dmg.DamageTaken), float64(dmg.Rounds))
	he, fire, flashed, smokes := grenadeMetrics(rows)
	v.HEDamage, v.FireDamage, v.Flashed, v.Smokes = he.Value, fire.Value, flashed.Value, smokes.Value
	if he.Value != nil && fire.Value != nil {
		v.GrenadeDamage = ptr(round2(*he.Value + *fire.Value))
	}
	if r := ext.RoundsT + ext.RoundsCT; r > 0 {
		v.SurvivedPct = ptr(math.Round(100 * float64(ext.SurvivedT+ext.SurvivedCT) / float64(r)))
	}
	for _, c := range clutches {
		if c.SteamID == steamID {
			v.ClutchAttempts += c.Count
			if c.Outcome == stats.ClutchWin {
				v.ClutchWins += c.Count
			}
		}
	}
	return v
}

func detailViews(rows []store.ExtRow, clutches []store.ClutchCount) []detailView {
	by := map[uint64][]store.ExtRow{}
	var order []uint64
	for _, r := range rows {
		if by[r.SteamID] == nil {
			order = append(order, r.SteamID)
		}
		by[r.SteamID] = append(by[r.SteamID], r)
	}
	out := make([]detailView, 0, len(order))
	for _, id := range order {
		out = append(out, newDetailView(id, by[id], clutches))
	}
	return out
}

type sessionExtResponse struct {
	extHeader
	Players []detailView `json:"players"`
}

func (s *Server) getSessionExt(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.GetSession(r.Context(), id); err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	rows, err := s.Store.SessionExtRows(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	clutches, err := s.Store.SessionClutches(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	// заголовок — по матчам, а не по строкам игроков
	seen := map[int64]bool{}
	var matches []store.ExtRow
	for _, row := range rows {
		if !seen[row.MatchID] {
			seen[row.MatchID] = true
			matches = append(matches, row)
		}
	}
	writeJSON(w, http.StatusOK, sessionExtResponse{extHeader: newExtHeader(matches), Players: detailViews(rows, clutches)})
}

// ── Матч ──

type killView struct {
	TimeSec       *float64 `json:"timeSec"`  // от начала раунда; null — начало неизвестно
	KillerID      string   `json:"killerId"` // "0" — мир
	VictimID      string   `json:"victimId"`
	Weapon        string   `json:"weapon"`
	Headshot      bool     `json:"headshot"`
	Trade         bool     `json:"trade"`
	ThroughSmoke  bool     `json:"throughSmoke"`
	Wallbang      bool     `json:"wallbang"`
	NoScope       bool     `json:"noScope"`
	AttackerBlind bool     `json:"attackerBlind"`
}

type clutchView struct {
	SteamID string              `json:"steamId"`
	Vs      int                 `json:"vs"`
	Outcome stats.ClutchOutcome `json:"outcome"`
}

type roundView struct {
	Number      int                 `json:"number"`
	Winner      parser.Team         `json:"winner"`
	SideA       parser.Side         `json:"sideA"`
	Reason      parser.Reason       `json:"reason"`
	DurationSec *float64            `json:"durationSec"`
	Restored    bool                `json:"restored"`
	ScoreA      int                 `json:"scoreA"` // счёт после раунда по записанным раундам
	ScoreB      int                 `json:"scoreB"`
	Clutches    []clutchView        `json:"clutches"`
	Kills       []killView          `json:"kills"`
	Alive       map[string][]string `json:"alive"` // команда → кто дожил до конца раунда
}

type matchExtResponse struct {
	Status  string `json:"status"` // complete | unavailable
	Quality struct {
		FirstRound     int  `json:"firstRound"`
		RestoredRound  bool `json:"restoredRound"`
		NoDamageEvents bool `json:"noDamageEvents"`
		NoFlashEvents  bool `json:"noFlashEvents"`
	} `json:"quality"`
	DurationSec *float64     `json:"durationSec"`
	ExtRounds   int          `json:"extRounds"` // раунды подробных колонок
	Rounds      []roundView  `json:"rounds"`
	Players     []detailView `json:"players"`
	Weapons     []weaponView `json:"weapons"`
}

func (s *Server) getMatchExt(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	e, err := s.Store.GetMatchExt(r.Context(), id)
	if err != nil {
		s.storeError(w, err, "матч не найден")
		return
	}
	resp := matchExtResponse{Status: duelsUnavailable, Rounds: []roundView{}, Players: []detailView{}, Weapons: []weaponView{}}
	if !e.Covered {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.Status = duelsComplete
	resp.Quality.FirstRound, resp.Quality.RestoredRound = e.FirstRound, e.RestoredRound
	resp.Quality.NoDamageEvents, resp.Quality.NoFlashEvents = !e.HasDamageEvents, !e.HasFlashEvents
	if e.Duration > 0 {
		resp.DurationSec = ptr(math.Round(e.Duration.Seconds()))
	}

	team := map[uint64]parser.Team{}
	for _, p := range e.Players {
		team[p.SteamID] = parser.Team(p.Team)
		if resp.ExtRounds == 0 && p.Covered {
			resp.ExtRounds = p.Ext.Rounds
		}
	}
	kills := map[int][]stats.KillEvent{}
	for _, k := range e.Kills {
		kills[k.Round] = append(kills[k.Round], k)
	}
	clutches := map[int][]clutchView{}
	for _, c := range e.Clutches {
		clutches[c.Round] = append(clutches[c.Round], clutchView{SteamID: strconv.FormatUint(c.SteamID, 10), Vs: c.Vs, Outcome: c.Outcome})
	}
	var scoreA, scoreB int
	for _, rd := range e.Rounds {
		if rd.Winner == parser.TeamA {
			scoreA++
		} else {
			scoreB++
		}
		v := roundView{
			Number: rd.Number, Winner: rd.Winner, SideA: rd.SideA, Reason: rd.Reason, Restored: rd.Restored,
			ScoreA: scoreA, ScoreB: scoreB, Clutches: clutches[rd.Number], Kills: []killView{},
		}
		if v.Clutches == nil {
			v.Clutches = []clutchView{}
		}
		if rd.Start > 0 && rd.End > rd.Start {
			v.DurationSec = ptr(math.Round((rd.End - rd.Start).Seconds()))
		}
		dead := map[uint64]bool{}
		for _, k := range kills[rd.Number] {
			kv := killView{
				KillerID: strconv.FormatUint(k.Killer, 10), VictimID: strconv.FormatUint(k.Victim, 10), Weapon: k.Weapon,
				Headshot: k.Headshot, Trade: k.Trade, ThroughSmoke: k.ThroughSmoke, Wallbang: k.Wallbang,
				NoScope: k.NoScope, AttackerBlind: k.AttackerBlind,
			}
			if rd.Start > 0 {
				kv.TimeSec = ptr(math.Max(0, math.Round((k.Time - rd.Start).Seconds())))
			}
			v.Kills = append(v.Kills, kv)
			if rd.End == 0 || k.Time <= rd.End {
				dead[k.Victim] = true
			}
		}
		if !rd.Restored {
			v.Alive = map[string][]string{"A": {}, "B": {}}
			for _, p := range e.Players {
				if !dead[p.SteamID] {
					t := string(team[p.SteamID])
					v.Alive[t] = append(v.Alive[t], strconv.FormatUint(p.SteamID, 10))
				}
			}
		}
		resp.Rounds = append(resp.Rounds, v)
	}

	clutchCounts := make([]store.ClutchCount, 0, len(e.Clutches))
	for _, c := range e.Clutches {
		clutchCounts = append(clutchCounts, store.ClutchCount{SteamID: c.SteamID, Vs: c.Vs, Outcome: c.Outcome, Count: 1})
	}
	resp.Players = detailViews(e.Players, clutchCounts)
	for _, wt := range e.Weapons {
		v := newWeaponView(wt)
		v.SteamID = strconv.FormatUint(wt.SteamID, 10)
		resp.Weapons = append(resp.Weapons, v)
	}
	writeJSON(w, http.StatusOK, resp)
}
