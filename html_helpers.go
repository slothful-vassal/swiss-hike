//go:build tinygo

package main

import "strings"

func inputValue(htmlText string, name string) string {
	for _, tag := range htmlTags(htmlText, "input") {
		if htmlAttr(tag, "name") == name {
			return htmlAttr(tag, "value")
		}
	}
	return ""
}

func metaRefreshURL(htmlText string) string {
	for _, tag := range htmlTags(htmlText, "meta") {
		if !strings.EqualFold(htmlAttr(tag, "http-equiv"), "refresh") {
			continue
		}
		content := htmlAttr(tag, "content")
		index := strings.Index(strings.ToLower(content), "url=")
		if index < 0 {
			continue
		}
		value := strings.TrimSpace(content[index+len("url="):])
		return strings.Trim(value, `"'`)
	}
	return ""
}

func htmlTags(htmlText string, tagName string) []string {
	lower := strings.ToLower(htmlText)
	needle := "<" + strings.ToLower(tagName)
	var tags []string
	for start := strings.Index(lower, needle); start >= 0; {
		nameEnd := start + len(needle)
		if nameEnd < len(lower) && isHTMLNameChar(lower[nameEnd]) {
			next := strings.Index(lower[nameEnd:], needle)
			if next < 0 {
				break
			}
			start = nameEnd + next
			continue
		}
		end := strings.Index(lower[start:], ">")
		if end < 0 {
			break
		}
		tags = append(tags, htmlText[start:start+end+1])
		next := strings.Index(lower[start+end+1:], needle)
		if next < 0 {
			break
		}
		start = start + end + 1 + next
	}
	return tags
}

func htmlAttr(tag string, key string) string {
	i := 0
	for i < len(tag) {
		for i < len(tag) && !isHTMLNameStart(tag[i]) {
			i++
		}
		start := i
		for i < len(tag) && isHTMLNameChar(tag[i]) {
			i++
		}
		if start == i {
			continue
		}
		name := tag[start:i]
		for i < len(tag) && isHTMLSpace(tag[i]) {
			i++
		}
		if i >= len(tag) || tag[i] != '=' {
			continue
		}
		i++
		for i < len(tag) && isHTMLSpace(tag[i]) {
			i++
		}
		if i >= len(tag) {
			break
		}
		value := ""
		if tag[i] == '"' || tag[i] == '\'' {
			quote := tag[i]
			i++
			valueStart := i
			for i < len(tag) && tag[i] != quote {
				i++
			}
			value = tag[valueStart:i]
			if i < len(tag) {
				i++
			}
		} else {
			valueStart := i
			for i < len(tag) && !isHTMLSpace(tag[i]) && tag[i] != '>' {
				i++
			}
			value = tag[valueStart:i]
		}
		if strings.EqualFold(name, key) {
			return htmlUnescape(value)
		}
	}
	return ""
}

func isHTMLNameStart(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '_' || char == ':'
}

func isHTMLNameChar(char byte) bool {
	return isHTMLNameStart(char) || (char >= '0' && char <= '9') || char == '-'
}

func isHTMLSpace(char byte) bool {
	return char == ' ' || char == '\n' || char == '\r' || char == '\t' || char == '\f'
}
