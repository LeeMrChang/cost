package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// LedgerPatch changes presentation fields only; ownership is resolved from the session.
type LedgerPatch struct {
	Name *string `json:"name"`
	Icon *string `json:"icon"`
}

func (p LedgerPatch) Normalize() (LedgerPatch, error) {
	if p.Name == nil && p.Icon == nil {
		return p, ErrInvalid
	}
	for _, field := range []struct {
		value **string
		limit int
	}{{&p.Name, 64}, {&p.Icon, 32}} {
		if *field.value == nil {
			continue
		}
		raw := **field.value
		if !utf8.ValidString(raw) {
			return p, ErrInvalid
		}
		for _, r := range raw {
			if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
				return p, ErrInvalid
			}
		}
		value := strings.TrimSpace(raw)
		if value == "" || utf8.RuneCountInString(value) > field.limit {
			return p, ErrInvalid
		}
		*field.value = &value
	}
	return p, nil
}
