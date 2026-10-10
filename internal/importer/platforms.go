package importer

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// userAgent — обычный браузерный: API платформ рассчитаны на свои страницы.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

// platformTimeout — сколько ждать ответа API платформы.
const platformTimeout = 15 * time.Second

const manualHint = " Загрузите файл демки вручную."

// Platforms — поддерживаемые платформы в порядке перечисления в текстах.
func Platforms() []Platform { return []Platform{NewFastCup(), NewCybershoke()} }

// postJSON отправляет JSON в API платформы и разбирает ответ в out.
func postJSON(parent context.Context, c *http.Client, title, endpoint string, body, out any) error {
	ctx, cancel := context.WithTimeout(parent, platformTimeout)
	defer cancel()
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		if parent.Err() != nil { // загрузку отменили — это не ошибка платформы
			return parent.Err()
		}
		return userErr("Сайт " + title + " не ответил. Попробуйте позже." + manualHint)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return userErr("Сайт " + title + " ответил ошибкой " + strconv.Itoa(resp.StatusCode) + ". Попробуйте позже." + manualHint)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return changedAPI(title)
	}
	return nil
}

func changedAPI(title string) error {
	return userErr(title + " изменил формат ответа, получить демку не удалось." + manualHint)
}

func notFound(title string) error { return userErr("Матч не найден на " + title + ".") }

var errNotFinished = userErr("Матч ещё не закончен. Добавьте ссылку после конца матча.")
var errNotReady = userErr("Демку ещё не выложили. Добавьте ссылку позже." + manualHint)

// ---------- FastCup ----------

//go:embed fastcup_get_match.graphql
var fastcupGetMatch string

// FastCup — cs2.fastcup.net. Данные матча — из того же GraphQL-запроса, что делает страница матча.
// Hasura FastCup пропускает только запросы из allowlist, поэтому текст запроса хранится дословно.
type FastCup struct {
	Endpoint string
	Now      func() time.Time
}

func NewFastCup() *FastCup {
	return &FastCup{Endpoint: "https://hasura.fastcup.net/v1/graphql", Now: time.Now}
}

func (*FastCup) Name() string              { return "fastcup" }
func (*FastCup) Title() string             { return "FastCup" }
func (*FastCup) Example() string           { return "https://cs2.fastcup.net/matches/<номер>" }
func (*FastCup) MatchURL(id string) string { return "https://cs2.fastcup.net/matches/" + id }
func (*FastCup) PageHosts() []string       { return []string{"cs2.fastcup.net", "fastcup.net"} }
func (*FastCup) Hosts() []string           { return []string{"fastcup.net"} }

func (p *FastCup) MatchID(u *url.URL) (string, error) {
	seg := pathSegments(u)
	if len(seg) >= 2 && seg[0] == "matches" && digits.MatchString(seg[1]) {
		return seg[1], nil
	}
	return "", pageError(p, seg)
}

func (p *FastCup) Resolve(ctx context.Context, c *http.Client, id string) (Match, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Match{}, notFound(p.Title())
	}
	var resp struct {
		Data *struct {
			Match *struct {
				Status               string     `json:"status"`
				StartedAt            *time.Time `json:"startedAt"`
				ReplayExpirationDate *time.Time `json:"replayExpirationDate"`
				Maps                 []struct {
					Number    int        `json:"number"`
					StartedAt *time.Time `json:"startedAt"`
					Replays   []struct {
						URL       string    `json:"url"`
						CreatedAt time.Time `json:"createdAt"`
					} `json:"replays"`
				} `json:"maps"`
			} `json:"match"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	body := map[string]any{
		"operationName": "__GetMatch",
		"variables":     map[string]any{"matchId": n, "gameId": 3},
		"query":         fastcupGetMatch,
	}
	if err := postJSON(ctx, c, p.Title(), p.Endpoint, body, &resp); err != nil {
		return Match{}, err
	}
	if len(resp.Errors) > 0 || resp.Data == nil {
		// в том числе «query is not allowed»: FastCup обновил запрос страницы
		return Match{}, changedAPI(p.Title())
	}
	m := resp.Data.Match
	if m == nil {
		return Match{}, notFound(p.Title())
	}
	if m.Status != "FINISHED" {
		return Match{}, errNotFinished
	}

	maps := m.Maps
	sort.SliceStable(maps, func(i, j int) bool { return maps[i].Number < maps[j].Number })
	out := Match{Number: n}
	for _, mp := range maps {
		if len(mp.Replays) == 0 {
			continue
		}
		last := mp.Replays[0]
		for _, r := range mp.Replays[1:] {
			if r.CreatedAt.After(last.CreatedAt) {
				last = r
			}
		}
		at := p.Now()
		switch {
		case mp.StartedAt != nil:
			at = *mp.StartedAt
		case m.StartedAt != nil:
			at = *m.StartedAt
		}
		name := "fastcup_" + id + ".dem"
		if u, err := url.Parse(last.URL); err == nil && path.Ext(u.Path) != "" {
			name = path.Base(u.Path)
		}
		out.Demos = append(out.Demos, Demo{URL: last.URL, Name: name, Maps: []MapInfo{{Number: mp.Number, StartedAt: at}}})
	}
	if len(out.Demos) == 0 {
		if m.ReplayExpirationDate != nil && m.ReplayExpirationDate.Before(p.Now()) {
			return Match{}, userErr("FastCup удалил демку: она хранится около 30 дней. Загрузите файл вручную, если он сохранён.")
		}
		return Match{}, errNotReady
	}
	out.StartedAt = out.Demos[0].Maps[0].StartedAt
	return out, nil
}

// ---------- Cybershoke ----------

// Cybershoke — cybershoke.net. Демка всего лобби (для серии — zip со всеми картами) отдаётся одной ссылкой.
type Cybershoke struct {
	Endpoint string
}

func NewCybershoke() *Cybershoke {
	return &Cybershoke{Endpoint: "https://cybershoke.net/api/api/v1/custom-matches/lobbys/info"}
}

func (*Cybershoke) Name() string              { return "cybershoke" }
func (*Cybershoke) Title() string             { return "Cybershoke" }
func (*Cybershoke) Example() string           { return "https://cybershoke.net/match/<номер>" }
func (*Cybershoke) MatchURL(id string) string { return "https://cybershoke.net/match/" + id }
func (*Cybershoke) PageHosts() []string       { return []string{"cybershoke.net"} }
func (*Cybershoke) Hosts() []string           { return []string{"cybershoke.net"} }

var langSegment = regexp.MustCompile(`^[a-z]{2}(-[a-z]{2})?$`)

func (p *Cybershoke) MatchID(u *url.URL) (string, error) {
	seg := pathSegments(u)
	if len(seg) > 0 && langSegment.MatchString(seg[0]) {
		seg = seg[1:]
	}
	if len(seg) >= 2 && seg[0] == "match" && digits.MatchString(seg[1]) {
		return seg[1], nil
	}
	return "", pageError(p, seg)
}

func (p *Cybershoke) Resolve(ctx context.Context, c *http.Client, id string) (Match, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Match{}, notFound(p.Title())
	}
	var resp struct {
		Result string `json:"result"`
		Data   *struct {
			MatchStats struct {
				// карты серии; пустое значение PHP отдаёт массивом [], поэтому разбираем вручную
				Bo json.RawMessage `json:"bo"`
			} `json:"match_stats"`
			Dates struct {
				Start *int64 `json:"unixtime_start_match"`
				End   *int64 `json:"unixtime_match_end"`
			} `json:"dates"`
			Demo *struct {
				Status      int    `json:"status"`
				URLDownload string `json:"url_download"`
			} `json:"demo"`
		} `json:"data"`
	}
	if err := postJSON(ctx, c, p.Title(), p.Endpoint, map[string]int64{"id_lobby": n}, &resp); err != nil {
		return Match{}, err
	}
	if resp.Result != "success" || resp.Data == nil {
		return Match{}, notFound(p.Title())
	}
	d := resp.Data
	if d.Dates.End == nil || *d.Dates.End == 0 {
		return Match{}, errNotFinished
	}
	if d.Demo == nil || d.Demo.Status != 4 || d.Demo.URLDownload == "" {
		return Match{}, errNotReady
	}

	var bo map[string]struct {
		Start int64 `json:"unixtime_start"`
	}
	_ = json.Unmarshal(d.MatchStats.Bo, &bo) // [] или мусор — карт нет, ниже запасной вариант
	var maps []MapInfo
	for k, v := range bo {
		num, err := strconv.Atoi(k)
		if err != nil || v.Start <= 0 {
			continue
		}
		maps = append(maps, MapInfo{Number: num, StartedAt: time.Unix(v.Start, 0).UTC()})
	}
	sort.Slice(maps, func(i, j int) bool { return maps[i].Number < maps[j].Number })
	if len(maps) == 0 {
		start := *d.Dates.End
		if d.Dates.Start != nil && *d.Dates.Start > 0 {
			start = *d.Dates.Start
		}
		maps = []MapInfo{{Number: 1, StartedAt: time.Unix(start, 0).UTC()}}
	}
	return Match{
		Number:    n,
		StartedAt: maps[0].StartedAt,
		Demos:     []Demo{{URL: d.Demo.URLDownload, Name: "match_" + id + ".zip", Maps: maps}},
	}, nil
}
