package main

import "testing"

func TestParseReparseArgs(t *testing.T) {
	ok := map[string]struct {
		args []string
		want reparseTarget
	}{
		"all":     {[]string{"--all"}, reparseTarget{all: true}},
		"match":   {[]string{"--match", "5"}, reparseTarget{match: 5}},
		"session": {[]string{"-session=2"}, reparseTarget{session: 2}},
	}
	for name, c := range ok {
		got, err := parseReparseArgs(c.args)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v err=%v", name, got, err)
		}
	}
	for name, args := range map[string][]string{
		"без параметров":   nil,
		"два параметра":    {"--all", "--match", "1"},
		"нулевой id":       {"--match", "0"},
		"отрицательный id": {"--session", "-3"},
		"не число":         {"--match", "abc"},
		"лишний аргумент":  {"--all", "extra"},
		"неизвестный флаг": {"--everything"},
	} {
		if _, err := parseReparseArgs(args); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}
