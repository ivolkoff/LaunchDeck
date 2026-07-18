package i18n

import "testing"

func TestParseThreeValued(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"ru", Ru, true},
		{"RU", Ru, true},
		{"ru_RU.UTF-8", Ru, true},
		{"russian", Ru, true},
		{"en", En, true},
		{"en_US.UTF-8", En, true},
		{"", En, false},
		{"fr", En, false},
		{"xx", En, false},
		{"e", En, false},
	}
	for _, c := range cases {
		got, ok := parse(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parse(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDetectPrecedence(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	// config override wins over env
	if got := Detect(env(map[string]string{"LANG": "en_US"}), "ru"); got != Ru {
		t.Errorf("cfg override: got %v want Ru", got)
	}
	// invalid config falls through to env
	if got := Detect(env(map[string]string{"LANG": "ru_RU"}), "fr"); got != Ru {
		t.Errorf("cfg invalid → env: got %v want Ru", got)
	}
	// LC_ALL beats LANG
	if got := Detect(env(map[string]string{"LC_ALL": "ru_RU", "LANG": "en_US"}), ""); got != Ru {
		t.Errorf("LC_ALL precedence: got %v want Ru", got)
	}
	// unset LC_ALL, LANG=ru wins (the critical regression: empty must not win as En)
	if got := Detect(env(map[string]string{"LANG": "ru_RU"}), ""); got != Ru {
		t.Errorf("empty LC_ALL then LANG=ru: got %v want Ru", got)
	}
	// nothing set → En
	if got := Detect(env(map[string]string{}), ""); got != En {
		t.Errorf("default: got %v want En", got)
	}
	// unknown everywhere → En
	if got := Detect(env(map[string]string{"LC_ALL": "fr", "LANG": "de"}), "xx"); got != En {
		t.Errorf("unknown → En: got %v want En", got)
	}
}

func TestTFallback(t *testing.T) {
	SetLang(En)
	t.Cleanup(func() { SetLang(En) })
	// unknown key → the key itself
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key: got %q", got)
	}
	// known key, English
	if got := T("list.empty"); got != "No matching services" {
		t.Errorf("en list.empty: got %q", got)
	}
	// Russian
	SetLang(Ru)
	if got := T("list.empty"); got != "Нет подходящих сервисов" {
		t.Errorf("ru list.empty: got %q", got)
	}
	// Tf formatting
	SetLang(En)
	if got := Tf("status.ok", "restart"); got != "restart ok" {
		t.Errorf("Tf en: got %q", got)
	}
}

// TestCatalogComplete guards the two silent-failure modes: an empty en (would
// show the raw key) or empty ru (would show half-English Russian UI).
func TestCatalogComplete(t *testing.T) {
	for k, e := range catalog {
		if e.en == "" {
			t.Errorf("catalog[%q].en is empty", k)
		}
		if e.ru == "" {
			t.Errorf("catalog[%q].ru is empty", k)
		}
	}
}
