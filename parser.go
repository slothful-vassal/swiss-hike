//go:build tinygo

package main

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func parseHikeHTML(body []byte) (hikePage, error) {
	root, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return hikePage{}, err
	}
	main := firstElement(root, "main", nil)
	if main == nil {
		return hikePage{}, fmt.Errorf("page has no main element")
	}

	page := hikePage{
		Title:       extractTitle(root),
		Description: extractDescription(main),
		Photos:      extractPhotos(main),
		GPXURL:      absolutizeURL(extractGPXURL(main)),
	}
	metadata := extractMetadata(main)
	page.DurationSeconds = metadata.DurationSeconds
	page.DistanceMeters = metadata.DistanceMeters
	page.ElevationGainMeters = metadata.ElevationGainMeters
	page.ElevationLossMeters = metadata.ElevationLossMeters
	page.Difficulty = metadata.Difficulty
	return page, nil
}

func extractTitle(root *html.Node) string {
	title := firstElement(root, "title", nil)
	if title == nil {
		return ""
	}
	return normalizeText(textContent(title))
}

func extractDescription(main *html.Node) string {
	section := firstElement(main, "section", func(n *html.Node) bool {
		return attr(n, "id") == "content_hike"
	})
	if section == nil {
		return ""
	}
	content := firstElement(section, "div", func(n *html.Node) bool {
		return hasClass(n, "l-hike-content")
	})
	if content == nil {
		return ""
	}
	textBlock := firstElement(content, "div", func(n *html.Node) bool {
		return hasClass(n, "l-text")
	})
	if textBlock != nil {
		return normalizeText(textContent(textBlock))
	}
	return normalizeText(textContent(content))
}

func extractPhotos(main *html.Node) []pagePhoto {
	slideshow := firstElement(main, "ul", func(n *html.Node) bool {
		return hasClass(n, "uk-slideshow-items")
	})
	if slideshow == nil {
		return nil
	}

	photos := make([]pagePhoto, 0)
	for _, anchor := range allElements(slideshow, "a", func(n *html.Node) bool {
		return attr(n, "href") != ""
	}) {
		caption := strings.TrimSpace(attr(anchor, "data-caption"))
		if caption == "" {
			if img := firstElement(anchor, "img", nil); img != nil {
				caption = strings.TrimSpace(attr(img, "alt"))
			}
		}
		photos = append(photos, pagePhoto{URL: absolutizeURL(attr(anchor, "href")), Caption: normalizeText(caption)})
	}
	if len(photos) == 0 {
		for _, img := range allElements(slideshow, "img", func(n *html.Node) bool {
			return attr(n, "src") != ""
		}) {
			photos = append(photos, pagePhoto{URL: absolutizeURL(attr(img, "src")), Caption: normalizeText(attr(img, "alt"))})
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

func extractGPXURL(main *html.Node) string {
	for _, tile := range allElements(main, "div", func(n *html.Node) bool {
		return hasClass(n, "ct-download-tile")
	}) {
		for _, anchor := range allElements(tile, "a", func(n *html.Node) bool {
			return attr(n, "href") != ""
		}) {
			href := attr(anchor, "href")
			if strings.HasSuffix(strings.ToLower(href), ".gpx") {
				return href
			}
		}
	}
	return ""
}

type metadataValues struct {
	DurationSeconds     *float64
	DistanceMeters      *float64
	ElevationGainMeters *float64
	ElevationLossMeters *float64
	Difficulty          string
}

func extractMetadata(main *html.Node) metadataValues {
	container := firstElement(main, "div", func(n *html.Node) bool {
		return hasClass(n, "l-metadata")
	})
	if container == nil {
		return metadataValues{}
	}
	values := metadataValues{}
	for _, item := range allElements(container, "", func(n *html.Node) bool {
		return attr(n, "uk-tooltip") != ""
	}) {
		tooltip := strings.TrimSpace(attr(item, "uk-tooltip"))
		text := normalizeText(textContent(item))
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

func firstElement(root *html.Node, tag string, match func(*html.Node) bool) *html.Node {
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && (tag == "" || child.Data == tag) && (match == nil || match(child)) {
			return child
		}
		if found := firstElement(child, tag, match); found != nil {
			return found
		}
	}
	return nil
}

func allElements(root *html.Node, tag string, match func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (tag == "" || n.Data == tag) && (match == nil || match(n)) {
			result = append(result, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return result
}

func attr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return htmlUnescape(attr.Val)
		}
	}
	return ""
}

func hasClass(n *html.Node, className string) bool {
	for _, part := range strings.Fields(attr(n, "class")) {
		if part == className {
			return true
		}
	}
	return false
}

func textContent(n *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			builder.WriteByte('\n')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
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
	return strings.Join(strings.Fields(strings.Join(parts, "\n")), " ")
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
