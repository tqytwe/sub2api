package service

import "regexp"

var (
	announcementRawHTMLPattern       = regexp.MustCompile(`(?is)<\s*/?\s*[a-z][^>]*>`)
	announcementMarkdownImagePattern = regexp.MustCompile(`!\[[^\]]*]\(\s*([^)\s]+)(?:\s+["'][^"']*["'])?\s*\)`)
)
