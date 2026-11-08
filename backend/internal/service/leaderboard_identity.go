package service

import (
	"net/mail"
	"strings"
	"unicode"
)

// MaskLeaderboardEmail is the only public leaderboard identity. Neither the
// complete local name nor the provider name leaves the public API boundary.
func MaskLeaderboardEmail(value string) string {
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || strings.ContainsFunc(value, unicode.IsSpace) {
		return ""
	}
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return ""
	}
	local, domain := []rune(parts[0]), strings.Split(parts[1], ".")
	if len(local) == 0 || len(domain) < 2 {
		return ""
	}
	for _, label := range domain {
		if label == "" {
			return ""
		}
	}
	n := 1
	if len(local) > 2 {
		n = 2
	}
	provider := []rune(domain[0])
	return string(local[:n]) + "***@" + string(provider[:1]) + "***." + domain[len(domain)-1]
}
