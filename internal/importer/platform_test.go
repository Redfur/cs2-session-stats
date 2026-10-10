package importer

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	ps := Platforms()
	ok := []struct{ in, platform, id, url string }{
		{"https://cs2.fastcup.net/matches/27802385", "fastcup", "27802385", "https://cs2.fastcup.net/matches/27802385"},
		{"  http://www.cs2.fastcup.net/matches/27802385/stats?tab=duels#top ", "fastcup", "27802385", "https://cs2.fastcup.net/matches/27802385"},
		{"cs2.fastcup.net/matches/27802385", "fastcup", "27802385", "https://cs2.fastcup.net/matches/27802385"},
		{"https://fastcup.net/matches/1", "fastcup", "1", "https://cs2.fastcup.net/matches/1"},
		{"https://cybershoke.net/match/12552678", "cybershoke", "12552678", "https://cybershoke.net/match/12552678"},
		{"https://cybershoke.net/ru/match/12578063?tab=stats", "cybershoke", "12578063", "https://cybershoke.net/match/12578063"},
		{"https://www.cybershoke.net/de/match/12552678/", "cybershoke", "12552678", "https://cybershoke.net/match/12552678"},
	}
	for _, c := range ok {
		l, err := ParseURL(ps, c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if l.Platform.Name() != c.platform || l.ID != c.id || l.URL != c.url {
			t.Errorf("%q: %s %s %s", c.in, l.Platform.Name(), l.ID, l.URL)
		}
	}

	bad := []struct{ in, want string }{
		{"https://cs2.fastcup.net/profile/1873320", "Это ссылка на профиль, а не на матч. Нужна страница матча, например https://cs2.fastcup.net/matches/<номер>"},
		{"https://cs2.fastcup.net/id1873320", "Это ссылка на профиль"},
		{"https://cybershoke.net/ru/profile/76561198000000000", "Это ссылка на профиль, а не на матч. Нужна страница матча, например https://cybershoke.net/match/<номер>"},
		{"https://cs2.fastcup.net/matches", "Это не страница матча"},
		{"https://cs2.fastcup.net/matches/abc", "Это не страница матча"},
		{"https://cybershoke.net/servers", "Это не страница матча"},
		{"https://replays.fastcup.net/16324690/27802385_24844407_2610071617-de_ancient.dem", "Это не страница матча"},
		{"https://cdn-de-1.cybershoke.net/demos/12552678?series=1", "Это не страница матча"},
		{"https://example.com/match/1", "Поддерживаются ссылки на матчи FastCup (https://cs2.fastcup.net/matches/<номер>) и Cybershoke (https://cybershoke.net/match/<номер>)"},
		{"https://www.faceit.com/ru/cs2/room/1-abc", "Поддерживаются ссылки на матчи"},
		{"https://fastcup.net.evil.com/matches/1", "Поддерживаются ссылки на матчи"},
		{"ftp://cs2.fastcup.net/matches/1", "Поддерживаются ссылки на матчи"},
		{"просто текст", "Поддерживаются ссылки на матчи"},
	}
	for _, c := range bad {
		_, err := ParseURL(ps, c.in)
		if err == nil || !strings.HasPrefix(UserMessage(err, ""), c.want) {
			t.Errorf("%q: ошибка %v, ожидалось начало %q", c.in, err, c.want)
		}
	}
}

func TestSplitURLs(t *testing.T) {
	got := SplitURLs(" a\nb\t\tc  a\r\n\nb ")
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("%v, ожидалось %v", got, want)
	}
	if got := SplitURLs(" \n "); len(got) != 0 {
		t.Fatalf("пустой ввод: %v", got)
	}
}

func TestHostAllowed(t *testing.T) {
	domains := []string{"fastcup.net", "cybershoke.net"}
	for host, want := range map[string]bool{
		"fastcup.net": true, "replays.fastcup.net": true, "CDN-DE-1.cybershoke.net": true,
		"evilfastcup.net": false, "fastcup.net.evil.com": false, "127.0.0.1": false,
	} {
		if HostAllowed(host, domains) != want {
			t.Errorf("%s: ожидалось %v", host, want)
		}
	}
}
