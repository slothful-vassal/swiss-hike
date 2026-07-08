package main

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

type htmlBlock struct {
	StartTag string
	Inner    string
}

func parseHikeHTML(body []byte) (hikePage, error) {
	htmlText := string(body)
	main, ok := firstHTMLBlock(htmlText, "main", nil)
	if !ok {
		return hikePage{}, fmt.Errorf("page has no main element")
	}

	page := hikePage{
		Title:       extractTitle(htmlText),
		Description: extractDescription(main.Inner),
		Photos:      extractPhotos(main.Inner),
		GPXURL:      absolutizeURL(extractGPXURL(main.Inner)),
	}
	metadata := extractMetadata(main.Inner)
	page.DurationSeconds = metadata.DurationSeconds
	page.DistanceMeters = metadata.DistanceMeters
	page.ElevationGainMeters = metadata.ElevationGainMeters
	page.ElevationLossMeters = metadata.ElevationLossMeters
	page.Difficulty = metadata.Difficulty
	return page, nil
}

func extractTitle(htmlText string) string {
	title, ok := firstHTMLBlock(htmlText, "title", nil)
	if !ok {
		return ""
	}
	return normalizeText(htmlTextContent(title.Inner))
}

func extractDescription(main string) string {
	section, ok := firstHTMLBlock(main, "section", func(tag string) bool {
		return htmlAttr(tag, "id") == "content_hike"
	})
	if !ok {
		return ""
	}
	content, ok := firstHTMLBlock(section.Inner, "div", func(tag string) bool {
		return tagHasClass(tag, "l-hike-content")
	})
	if !ok {
		return ""
	}
	if textBlock, ok := firstHTMLBlock(content.Inner, "div", func(tag string) bool {
		return tagHasClass(tag, "l-text")
	}); ok {
		return normalizeText(htmlTextContent(textBlock.Inner))
	}
	return normalizeText(htmlTextContent(content.Inner))
}

func extractPhotos(main string) []pagePhoto {
	slideshow, ok := firstHTMLBlock(main, "ul", func(tag string) bool {
		return tagHasClass(tag, "uk-slideshow-items")
	})
	if !ok {
		return nil
	}

	photos := make([]pagePhoto, 0)
	for _, anchor := range htmlBlocks(slideshow.Inner, "a", func(tag string) bool {
		return htmlAttr(tag, "href") != ""
	}) {
		caption := strings.TrimSpace(htmlAttr(anchor.StartTag, "data-caption"))
		if caption == "" {
			for _, img := range htmlTags(anchor.Inner, "img") {
				caption = strings.TrimSpace(htmlAttr(img, "alt"))
				if caption != "" {
					break
				}
			}
		}
		photos = append(photos, pagePhoto{URL: absolutizeURL(htmlAttr(anchor.StartTag, "href")), Caption: normalizeText(caption)})
	}
	if len(photos) == 0 {
		for _, img := range htmlTags(slideshow.Inner, "img") {
			src := htmlAttr(img, "src")
			if src == "" {
				continue
			}
			photos = append(photos, pagePhoto{URL: absolutizeURL(src), Caption: normalizeText(htmlAttr(img, "alt"))})
		}
	}

	seen := map[string]bool{}
	deduped := make([]pagePhoto, 0, len(photos))
	for _, photo := range photos {
		if photo.URL == "" || seen[photo.URL] {
			continue
		}
		seen[photo.URL] = true
		deduped = append(deduped, photo)
	}
	return deduped
}

func extractGPXURL(main string) string {
	for _, tile := range htmlBlocks(main, "div", func(tag string) bool {
		return tagHasClass(tag, "ct-download-tile")
	}) {
		for _, anchor := range htmlTags(tile.Inner, "a") {
			href := htmlAttr(anchor, "href")
			if isGPXHref(href) {
				return href
			}
		}
	}
	return ""
}

func isGPXHref(href string) bool {
	parsed, err := url.Parse(href)
	if err == nil {
		return strings.HasSuffix(strings.ToLower(parsed.Path), ".gpx")
	}
	clean := href
	if index := strings.IndexAny(clean, "?#"); index >= 0 {
		clean = clean[:index]
	}
	return strings.HasSuffix(strings.ToLower(clean), ".gpx")
}

type metadataValues struct {
	DurationSeconds     *float64
	DistanceMeters      *float64
	ElevationGainMeters *float64
	ElevationLossMeters *float64
	Difficulty          string
}

func extractMetadata(main string) metadataValues {
	container, ok := firstHTMLBlock(main, "div", func(tag string) bool {
		return tagHasClass(tag, "l-metadata")
	})
	if !ok {
		return metadataValues{}
	}
	values := metadataValues{}
	for _, item := range htmlBlocksWithAttr(container.Inner, "uk-tooltip") {
		tooltip := strings.TrimSpace(htmlAttr(item.StartTag, "uk-tooltip"))
		text := normalizeText(htmlTextContent(item.Inner))
		switch tooltip {
		case "Wanderzeit":
			values.DurationSeconds = parseDurationSeconds(text)
		case "Distanz":
			values.DistanceMeters = parseDistanceMeters(text)
		case "Aufstieg":
			values.ElevationGainMeters = parseElevationMeters(text)
		case "Abstieg":
			values.ElevationLossMeters = parseElevationMeters(text)
		case "Körperliche Anforderung":
			switch strings.ToLower(strings.TrimSpace(text)) {
			case "tief":
				values.Difficulty = "easy"
			case "mittel":
				values.Difficulty = "moderate"
			case "hoch":
				values.Difficulty = "difficult"
			}
		}
	}
	return values
}

func firstHTMLBlock(htmlText string, tagName string, match func(string) bool) (htmlBlock, bool) {
	for _, block := range htmlBlocks(htmlText, tagName, match) {
		return block, true
	}
	return htmlBlock{}, false
}

func htmlBlocks(htmlText string, tagName string, match func(string) bool) []htmlBlock {
	lower := strings.ToLower(htmlText)
	tagName = strings.ToLower(tagName)
	blocks := make([]htmlBlock, 0)
	for search := 0; search < len(htmlText); {
		start := findHTMLTag(lower, tagName, false, search)
		if start < 0 {
			break
		}
		startEnd := htmlTagEnd(htmlText, start)
		if startEnd < 0 {
			break
		}
		startTag := htmlText[start:startEnd]
		search = startEnd
		if isHTMLSelfClosing(startTag) || isHTMLVoidTag(tagName) {
			continue
		}
		closeStart, _ := matchingHTMLClose(lower, tagName, startEnd)
		if closeStart < 0 {
			continue
		}
		if match == nil || match(startTag) {
			blocks = append(blocks, htmlBlock{StartTag: startTag, Inner: htmlText[startEnd:closeStart]})
		}
	}
	return blocks
}

func htmlBlocksWithAttr(htmlText string, attrName string) []htmlBlock {
	lower := strings.ToLower(htmlText)
	blocks := make([]htmlBlock, 0)
	for search := 0; search < len(htmlText); {
		start := strings.IndexByte(lower[search:], '<')
		if start < 0 {
			break
		}
		start += search
		tagName, closing := htmlTagNameAt(lower, start)
		if tagName == "" || closing {
			search = start + 1
			continue
		}
		startEnd := htmlTagEnd(htmlText, start)
		if startEnd < 0 {
			break
		}
		startTag := htmlText[start:startEnd]
		search = startEnd
		if htmlAttr(startTag, attrName) == "" || isHTMLSelfClosing(startTag) || isHTMLVoidTag(tagName) {
			continue
		}
		closeStart, _ := matchingHTMLClose(lower, tagName, startEnd)
		if closeStart < 0 {
			continue
		}
		blocks = append(blocks, htmlBlock{StartTag: startTag, Inner: htmlText[startEnd:closeStart]})
	}
	return blocks
}

func matchingHTMLClose(lower string, tagName string, from int) (int, int) {
	depth := 1
	for search := from; search < len(lower); {
		nextOpen := findHTMLTag(lower, tagName, false, search)
		nextClose := findHTMLTag(lower, tagName, true, search)
		if nextClose < 0 {
			return -1, -1
		}
		if nextOpen >= 0 && nextOpen < nextClose {
			openEnd := htmlTagEnd(lower, nextOpen)
			if openEnd < 0 {
				return -1, -1
			}
			if !isHTMLSelfClosing(lower[nextOpen:openEnd]) && !isHTMLVoidTag(tagName) {
				depth++
			}
			search = openEnd
			continue
		}
		closeEnd := htmlTagEnd(lower, nextClose)
		if closeEnd < 0 {
			return -1, -1
		}
		depth--
		if depth == 0 {
			return nextClose, closeEnd
		}
		search = closeEnd
	}
	return -1, -1
}

func findHTMLTag(lower string, tagName string, closing bool, start int) int {
	for search := start; search < len(lower); {
		index := strings.IndexByte(lower[search:], '<')
		if index < 0 {
			return -1
		}
		index += search
		foundName, foundClosing := htmlTagNameAt(lower, index)
		if foundName == tagName && foundClosing == closing {
			return index
		}
		search = index + 1
	}
	return -1
}

func htmlTagNameAt(lower string, start int) (string, bool) {
	if start < 0 || start >= len(lower) || lower[start] != '<' {
		return "", false
	}
	i := start + 1
	if i >= len(lower) || lower[i] == '!' || lower[i] == '?' {
		return "", false
	}
	closing := false
	if lower[i] == '/' {
		closing = true
		i++
	}
	for i < len(lower) && isHTMLSpace(lower[i]) {
		i++
	}
	if i >= len(lower) || !isHTMLNameStart(lower[i]) {
		return "", closing
	}
	nameStart := i
	for i < len(lower) && isHTMLNameChar(lower[i]) {
		i++
	}
	return lower[nameStart:i], closing
}

func htmlTagEnd(htmlText string, start int) int {
	end := strings.IndexByte(htmlText[start:], '>')
	if end < 0 {
		return -1
	}
	return start + end + 1
}

func isHTMLSelfClosing(tag string) bool {
	return strings.HasSuffix(strings.TrimSpace(tag), "/>")
}

func isHTMLVoidTag(tagName string) bool {
	switch tagName {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}

func tagHasClass(tag string, className string) bool {
	for _, part := range strings.Fields(htmlAttr(tag, "class")) {
		if part == className {
			return true
		}
	}
	return false
}

func htmlTextContent(htmlText string) string {
	var builder strings.Builder
	lower := strings.ToLower(htmlText)
	for i := 0; i < len(htmlText); {
		if htmlText[i] == '<' {
			tagName, closing := htmlTagNameAt(lower, i)
			end := htmlTagEnd(htmlText, i)
			if end < 0 {
				break
			}
			if !closing && (tagName == "script" || tagName == "style" || tagName == "svg") {
				if _, closeEnd := matchingHTMLClose(lower, tagName, end); closeEnd > 0 {
					i = closeEnd
					builder.WriteByte('\n')
					continue
				}
			}
			builder.WriteByte('\n')
			i = end
			continue
		}
		next := strings.IndexByte(htmlText[i:], '<')
		if next < 0 {
			next = len(htmlText) - i
		}
		text := htmlUnescape(htmlText[i : i+next])
		if strings.TrimSpace(text) != "" {
			builder.WriteString(text)
			builder.WriteByte('\n')
		}
		i += next
	}
	return builder.String()
}

func normalizeText(text string) string {
	lines := strings.Split(text, "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			parts = append(parts, line)
		}
	}
	return compactPunctuationSpacing(strings.Join(strings.Fields(strings.Join(parts, "\n")), " "))
}

func compactPunctuationSpacing(text string) string {
	replacer := strings.NewReplacer(
		" .", ".",
		" ,", ",",
		" ;", ";",
		" :", ":",
		" !", "!",
		" ?", "?",
		" )", ")",
		"( ", "(",
	)
	return replacer.Replace(text)
}

func htmlUnescape(value string) string {
	return html.UnescapeString(value)
}

func absolutizeURL(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() {
		return value
	}
	if strings.HasPrefix(value, "/") {
		return "https://www.schweizer-wanderwege.ch" + value
	}
	return "https://www.schweizer-wanderwege.ch/" + value
}

func parseNumber(value string) *float64 {
	raw := firstNumber(value)
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", "."), 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseDurationSeconds(value string) *float64 {
	text := strings.ToLower(value)
	hours := 0.0
	minutes := 0.0
	if parsed := numberBeforeUnit(text, "h"); parsed != nil {
		hours = *parsed
	}
	if parsed := numberBeforeUnit(text, "min"); parsed != nil {
		minutes = *parsed
	}
	if hours == 0 && minutes == 0 {
		return nil
	}
	seconds := hours*3600 + minutes*60
	return &seconds
}

func parseDistanceMeters(value string) *float64 {
	number := parseNumber(value)
	if number == nil {
		return nil
	}
	text := strings.ToLower(value)
	meters := *number
	if strings.Contains(text, "km") {
		meters *= 1000
	}
	return &meters
}

func parseElevationMeters(value string) *float64 {
	return parseNumber(value)
}

func firstNumber(value string) string {
	start := -1
	seenSeparator := false
	for index, char := range value {
		isDigit := char >= '0' && char <= '9'
		isSeparator := char == '.' || char == ','
		if start < 0 {
			if isDigit {
				start = index
			}
			continue
		}
		if isDigit {
			continue
		}
		if isSeparator && !seenSeparator {
			seenSeparator = true
			continue
		}
		return value[start:index]
	}
	if start < 0 {
		return ""
	}
	return value[start:]
}

func numberBeforeUnit(text string, unit string) *float64 {
	index := strings.Index(text, unit)
	for index >= 0 {
		if index+len(unit) < len(text) && isASCIIAlpha(text[index+len(unit)]) {
			next := strings.Index(text[index+len(unit):], unit)
			if next < 0 {
				return nil
			}
			index += len(unit) + next
			continue
		}
		end := index
		for end > 0 && text[end-1] == ' ' {
			end--
		}
		start := end
		seenSeparator := false
		for start > 0 {
			char := text[start-1]
			if char >= '0' && char <= '9' {
				start--
				continue
			}
			if (char == '.' || char == ',') && !seenSeparator {
				seenSeparator = true
				start--
				continue
			}
			break
		}
		if start < end {
			return parseNumber(text[start:end])
		}
		next := strings.Index(text[index+len(unit):], unit)
		if next < 0 {
			return nil
		}
		index += len(unit) + next
	}
	return nil
}

func isASCIIAlpha(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}
