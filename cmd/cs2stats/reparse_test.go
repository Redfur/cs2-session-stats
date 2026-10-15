package main

import (
	"strings"
	"testing"
)

func TestParseReparseArgs(t *testing.T) {
	ok := map[string]struct {
		args []string
		want reparseTarget
	}{
		"all":     {[]string{"--all"}, reparseTarget{all: true}},
		"match":   {[]string{"--match", "m_Vf8nR2pQxa7c"}, reparseTarget{match: "m_Vf8nR2pQxa7c"}},
		"session": {[]string{"-session=s_Vf8nR2pQxa7c"}, reparseTarget{session: "s_Vf8nR2pQxa7c"}},
	}
	for name, c := range ok {
		got, err := parseReparseArgs(c.args)
		if err != nil || got != c.want {
			t.Errorf("%s: %+v err=%v", name, got, err)
		}
	}
	for name, c := range map[string]struct {
		args []string
		want string // часть текста ошибки
	}{
		"без параметров":         {nil, "ровно один"},
		"два параметра":          {[]string{"--all", "--match", "m_Vf8nR2pQxa7c"}, "ровно один"},
		"числовой id матча":      {[]string{"--match", "5"}, "неверный id матча"},
		"числовой id сессии":     {[]string{"--session", "3"}, "неверный id сессии"},
		"чужой префикс у матча":  {[]string{"--match", "s_Vf8nR2pQxa7c"}, "неверный id матча"},
		"чужой префикс у сессии": {[]string{"--session", "m_Vf8nR2pQxa7c"}, "неверный id сессии"},
		"пустой id матча":        {[]string{"--match", ""}, "неверный id матча"},
		"пустой id сессии":       {[]string{"--session="}, "неверный id сессии"},
		"лишний аргумент":        {[]string{"--all", "extra"}, "лишние аргументы"},
		"неизвестный флаг":       {[]string{"--everything"}, "flag provided but not defined"},
	} {
		_, err := parseReparseArgs(c.args)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: ошибка %v, ожидалось «%s»", name, err, c.want)
		}
	}
}
