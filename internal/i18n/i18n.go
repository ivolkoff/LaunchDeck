// Package i18n holds LaunchDeck's in-binary EN/RU message catalog and the
// process-global current language. The language is chosen once at startup
// (see Detect) and read through T/Tf; it does not change during a run.
package i18n

import (
	"fmt"
	"strings"
)

type Lang int

const (
	En Lang = iota // default; also the zero value
	Ru
)

var current Lang // En (zero value) until SetLang

// SetLang sets the process-wide language. Production calls it once at startup,
// before any localized T/Tf call. Not safe for concurrent use with T/Tf.
func SetLang(l Lang) { current = l }

// T returns the message for key in the current language. A missing entry (or an
// empty value for the current language) falls back to English, then to the key
// itself, so a gap degrades visibly and never panics.
func T(key string) string {
	e, ok := catalog[key]
	if !ok {
		return key
	}
	if current == Ru && e.ru != "" {
		return e.ru
	}
	return e.en
}

// Tf is T followed by fmt.Sprintf with the current language's format string.
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// parse maps a locale-ish value to a language. ok is true only when the leading
// run of ASCII letters begins with exactly "ru" or "en" (case-insensitive);
// empty input or anything else is not ok.
func parse(s string) (Lang, bool) {
	i := 0
	for i < len(s) {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			i++
			continue
		}
		break
	}
	head := strings.ToLower(s[:i])
	switch {
	case strings.HasPrefix(head, "ru"):
		return Ru, true
	case strings.HasPrefix(head, "en"):
		return En, true
	}
	return En, false
}

// Detect resolves the language. Precedence: a valid cfgLang wins; else the first
// of LC_ALL, LANG, LANGUAGE that parses; else En. getenv is injected so tests
// can supply an environment (pass os.Getenv in production).
func Detect(getenv func(string) string, cfgLang string) Lang {
	if l, ok := parse(cfgLang); ok {
		return l
	}
	for _, k := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		if l, ok := parse(getenv(k)); ok {
			return l
		}
	}
	return En
}
