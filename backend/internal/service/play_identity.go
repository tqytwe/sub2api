package service

import (
	"strconv"
	"strings"
)

func PublicPlayDisplayName(username, email string, userID int64) string {
	if name := normalizedPlayUsername(username); name != "" {
		return name
	}
	if masked := MaskPlayEmail(email); masked != "" {
		return masked
	}
	return playUserFallback(userID)
}

func AdminPlayDisplayName(username, email string, userID int64) string {
	if name := normalizedPlayUsername(username); name != "" {
		return name
	}
	if trimmed := strings.TrimSpace(email); trimmed != "" {
		return trimmed
	}
	return playUserFallback(userID)
}

func MaskPlayEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return ""
	}
	local := email[:at]
	domain := email[at+1:]
	prefix := local
	if len(prefix) > 2 {
		prefix = prefix[:2]
	} else if len(prefix) > 1 {
		prefix = prefix[:1]
	}
	return prefix + "***@" + domain
}

func normalizedPlayUsername(username string) string {
	username = strings.TrimSpace(username)
	if username == "" || strings.EqualFold(username, "user") {
		return ""
	}
	return username
}

func playUserFallback(userID int64) string {
	if userID <= 0 {
		return "user"
	}
	return "user-" + strconv.FormatInt(userID, 10)
}
