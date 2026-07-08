package main

import "strings"

func normalizeLocationURL(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&#38;", "&",
		"&#x26;", "&",
		"&#X26;", "&",
	)
	return replacer.Replace(value)
}
