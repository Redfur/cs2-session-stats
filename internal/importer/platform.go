// Package importer импортирует матчи по ссылке на страницу матча платформы (FastCup, Cybershoke):
// разбирает ссылки, узнаёт у платформы адреса демок и в фоне скачивает их в сессию.
package importer

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// MapInfo — карта серии в файле демки.
type MapInfo struct {
	Number    int       // номер карты в серии, с 1
	StartedAt time.Time // время начала карты у платформы
}

// Demo — файл демки на платформе: отдельная карта (FastCup) или архив серии (Cybershoke).
type Demo struct {
	URL  string
	Name string    // имя для приёма, если сервер не назовёт файл сам
	Maps []MapInfo // карты в файле по порядку
}

// Match — итог Resolve: что платформа знает о матче.
type Match struct {
	Number    int64     // номер матча на платформе
	StartedAt time.Time // начало первой карты: порядок очереди
	Demos     []Demo
}

// Platform — платформа, с которой можно импортировать матчи.
type Platform interface {
	Name() string    // идентификатор: fastcup, cybershoke
	Title() string   // название для текстов: FastCup, Cybershoke
	Example() string // пример ссылки на матч
	MatchURL(id string) string
	// PageHosts — хосты страниц матчей; Hosts — домены, к которым разрешены запросы (с поддоменами).
	PageHosts() []string
	Hosts() []string
	// MatchID извлекает номер матча из пути ссылки на страницу платформы.
	MatchID(u *url.URL) (string, error)
	Resolve(ctx context.Context, c *http.Client, id string) (Match, error)
}

// Link — распознанная ссылка на матч.
type Link struct {
	Platform Platform
	ID       string // номер матча на платформе
	URL      string // нормализованная ссылка
}

// UserError — ошибка, текст которой можно показать пользователю как есть.
type UserError struct{ Msg string }

func (e *UserError) Error() string { return e.Msg }

func userErr(format string) error { return &UserError{Msg: format} }

// UserMessage возвращает текст для пользователя; внутренние ошибки не раскрываются.
func UserMessage(err error, fallback string) string {
	var ue *UserError
	if errors.As(err, &ue) {
		return ue.Msg
	}
	return fallback
}

// SplitURLs делит ввод на ссылки по пробелам и переводам строк, убирая повторы.
func SplitURLs(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Fields(text) {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

func unsupported(platforms []Platform) error {
	parts := make([]string, len(platforms))
	for i, p := range platforms {
		parts[i] = p.Title() + " (" + p.Example() + ")"
	}
	return userErr("Поддерживаются ссылки на матчи " + strings.Join(parts, " и "))
}

// ParseURL распознаёт ссылку на страницу матча одной из платформ.
func ParseURL(platforms []Platform, raw string) (Link, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Link{}, unsupported(platforms)
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	for _, p := range platforms {
		if !HostAllowed(host, p.Hosts()) {
			continue
		}
		if !hostIn(host, p.PageHosts()) {
			return Link{}, notMatchPage(p)
		}
		id, err := p.MatchID(u)
		if err != nil {
			return Link{}, err
		}
		return Link{Platform: p, ID: id, URL: p.MatchURL(id)}, nil
	}
	return Link{}, unsupported(platforms)
}

var digits = regexp.MustCompile(`^[0-9]{1,12}$`)

// pathSegments — непустые части пути.
func pathSegments(u *url.URL) []string {
	var out []string
	for _, s := range strings.Split(u.Path, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

var profilePrefixes = []string{"profile", "user", "users", "player", "players", "id"}

// pageError — ошибка для ссылки на сайт платформы, которая ведёт не на матч.
func pageError(p Platform, seg []string) error {
	if len(seg) > 0 {
		first := strings.ToLower(seg[0])
		for _, pre := range profilePrefixes {
			if first == pre || (pre == "id" && strings.HasPrefix(first, "id") && digits.MatchString(first[2:])) {
				return userErr("Это ссылка на профиль, а не на матч. Нужна страница матча, например " + p.Example())
			}
		}
	}
	return notMatchPage(p)
}

func notMatchPage(p Platform) error {
	return userErr("Это не страница матча. Нужна страница матча, например " + p.Example())
}

func hostIn(host string, hosts []string) bool {
	for _, h := range hosts {
		if host == h {
			return true
		}
	}
	return false
}

// HostAllowed — host совпадает с одним из доменов или является его поддоменом.
func HostAllowed(host string, domains []string) bool {
	host = strings.ToLower(host)
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
