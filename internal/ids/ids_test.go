package ids

import (
	"strings"
	"testing"
)

func TestNewFormat(t *testing.T) {
	for _, kind := range []Kind{Group, Session, Match, Import} {
		id := New(kind)
		if !strings.HasPrefix(id, string(kind)) || len(id) != len(kind)+Length {
			t.Fatalf("%s: неверный id %q", kind, id)
		}
		for _, c := range id[len(kind):] {
			if !strings.ContainsRune(alphabet, c) {
				t.Fatalf("%s: символ %q вне алфавита в %q", kind, c, id)
			}
		}
		if !Valid(kind, id) {
			t.Fatalf("%s: Valid(%q) = false", kind, id)
		}
	}
}

func TestNewUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 10000; i++ {
		id := New(Match)
		if seen[id] {
			t.Fatalf("повтор id %q на шаге %d", id, i)
		}
		seen[id] = true
	}
}

func TestValidRejects(t *testing.T) {
	for _, c := range []struct {
		kind Kind
		s    string
	}{
		{Session, ""},
		{Session, "3"},
		{Session, "12345678901234"},
		{Session, "s_"},
		{Session, "m_k3Xq9wTb2LpA"},  // чужой префикс
		{Group, "imp_k3Xq9wTb2LpA"},  // чужой префикс
		{Session, "s_k3Xq9wTb2Lp"},   // короче
		{Session, "s_k3Xq9wTb2LpAB"}, // длиннее
		{Session, "s_k3Xq9wTb2Lp-"},  // символ вне алфавита
		{Session, "s_k3Xq9wTb2LpЖ"},
		{Import, "IMP_k3Xq9wTb2LpA"},
	} {
		if Valid(c.kind, c.s) {
			t.Errorf("Valid(%s, %q) = true", c.kind, c.s)
		}
	}
	if !Valid(Group, "g_k3Xq9wTb2LpA") || !Valid(Import, "imp_000000000000") {
		t.Error("верный id отклонён")
	}
}
