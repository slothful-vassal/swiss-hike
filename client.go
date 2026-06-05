//go:build tinygo

package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/open-wanderer/wanderer/plugins/sdk"
)

const (
	providerID          = "swiss-hike"
	basePath            = "/de"
	detailPath          = "/de/wandervorschlaege/uuid/"
	jsonMaxBytes        = 16 * 1024 * 1024
	htmlMaxBytes        = 8 * 1024 * 1024
	gpxMaxBytes         = 16 * 1024 * 1024
	defaultPageSize     = 50
	defaultMaxPhotos    = -1
	defaultDebugLogging = false
)

func listHikes(input listInput) (listOutput, error) {
	ctx := requestContext{debugLogging: sdk.BoolOption(input.Options, "debugLogging", defaultDebugLogging)}
	startAll := time.Now()
	token, cookie, err := fetchToken()
	if err != nil {
		return listOutput{}, err
	}
	ctx.logElapsed("list fetch token", startAll, "")

	start := time.Now()
	body, err := getBodyWithHeaders("/de/map-filter", []sdk.QueryParam{
		{Name: "app_filter_map[_token]", Value: token},
	}, headersWithCookie([]string{"application/json", "text/json"}, cookie), jsonMaxBytes)
	if err != nil {
		return listOutput{}, err
	}
	ctx.logElapsed("list map-filter", start, fmt.Sprintf("bytes=%d", len(body)))

	start = time.Now()
	ids, err := extractHikeIDs(body)
	if err != nil {
		return listOutput{}, err
	}
	ctx.logElapsed("list parse ids", start, fmt.Sprintf("ids=%d", len(ids)))

	offset := sdk.IntState(input.State, "offset", 0)
	limit := sdk.SyncLimit(input)
	if limit <= 0 {
		limit = defaultPageSize
	}
	if offset > len(ids) {
		offset = len(ids)
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}

	items := make([]trailSummary, 0, end-offset)
	for _, id := range ids[offset:end] {
		items = append(items, trailSummary{
			Source: trailImportSource{
				Provider:   providerID,
				ExternalID: id,
				URL:        "https://www.schweizer-wanderwege.ch" + detailPath + url.PathEscape(id),
			},
			Kind: "planned",
		})
	}

	hasMore := end < len(ids)
	ctx.logElapsed("list complete", startAll, fmt.Sprintf("start=%d end=%d total=%d hasMore=%t", offset, end, len(ids), hasMore))
	return listOutput{
		Items:   items,
		State:   map[string]any{"offset": end, "hasMore": hasMore},
		HasMore: hasMore,
	}, nil
}

func hikeDetail(externalID string, auth map[string]any, options map[string]any) (trailImport, error) {
	ctx := requestContext{debugLogging: sdk.BoolOption(options, "debugLogging", defaultDebugLogging)}
	if externalID == "" {
		return trailImport{}, fmt.Errorf("empty hike id")
	}

	siteCookie := ""
	if sdk.StringField(auth, "email") != "" || sdk.StringField(auth, "password") != "" {
		start := time.Now()
		session, cached, err := loginSessionCached(ctx, auth)
		if err != nil {
			return trailImport{}, err
		}
		siteCookie = session.siteCookie
		if cached {
			ctx.logDebug("detail login cache hit", "externalID="+externalID)
		} else {
			ctx.logElapsed("detail login", start, "externalID="+externalID)
		}
	}

	start := time.Now()
	html, err := getBodyWithHeaders(detailPath+url.PathEscape(externalID), nil, headersWithCookie([]string{"text/html"}, siteCookie), htmlMaxBytes)
	if err != nil {
		return trailImport{}, err
	}
	ctx.logElapsed("detail html", start, fmt.Sprintf("externalID=%s bytes=%d", externalID, len(html)))
	if redirectPath := metaRefreshPath(html); redirectPath != "" {
		start = time.Now()
		html, err = getBodyWithHeaders(redirectPath, nil, headersWithCookie([]string{"text/html"}, siteCookie), htmlMaxBytes)
		if err != nil {
			return trailImport{}, err
		}
		ctx.logElapsed("detail redirect html", start, fmt.Sprintf("externalID=%s path=%s bytes=%d", externalID, redirectPath, len(html)))
	}
	start = time.Now()
	page, err := parseHikeHTML(html)
	if err != nil {
		return trailImport{}, err
	}
	ctx.logElapsed("detail parse html", start, "externalID="+externalID)
	if page.Description == "" {
		return trailImport{}, fmt.Errorf("hike %s has no importable description", externalID)
	}
	if page.GPXURL == "" {
		return trailImport{}, fmt.Errorf("hike %s has no GPX download", externalID)
	}
	start = time.Now()
	gpx, err := getProviderURLWithCookie(page.GPXURL, siteCookie, []string{"application/gpx+xml", "application/xml", "text/xml", "application/octet-stream"}, gpxMaxBytes)
	if err != nil {
		return trailImport{}, err
	}
	ctx.logElapsed("detail gpx", start, fmt.Sprintf("externalID=%s bytes=%d", externalID, len(gpx)))
	maxPhotos := sdk.IntOption(options, "maxPhotos", defaultMaxPhotos)
	return pageImport(externalID, page, gpx, maxPhotos), nil
}

func metaRefreshPath(body []byte) string {
	value := metaRefreshURL(string(body))
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		if parsed.Host != "www.schweizer-wanderwege.ch" {
			return ""
		}
		return parsed.EscapedPath()
	}
	if strings.HasPrefix(value, "/de/") {
		return value
	}
	return ""
}

func fetchToken() (string, string, error) {
	response, body, err := requestBody(basePath, nil, baseHeaders([]string{"text/html"}), htmlMaxBytes)
	if err != nil {
		return "", "", err
	}
	token := inputValue(string(body), "app_filter_map[_token]")
	if token == "" {
		return "", "", fmt.Errorf("could not find map CSRF token")
	}
	return token, cookieHeader(response), nil
}

func getBody(path string, query []sdk.QueryParam, contentTypes []string, maxBytes int64) ([]byte, error) {
	_, body, err := requestBody(path, query, baseHeaders(contentTypes), maxBytes)
	return body, err
}

func getBodyWithHeaders(path string, query []sdk.QueryParam, headers map[string]string, maxBytes int64) ([]byte, error) {
	_, body, err := requestBody(path, query, headers, maxBytes)
	return body, err
}

func requestBody(path string, query []sdk.QueryParam, headers map[string]string, maxBytes int64) (sdk.HostResponse, []byte, error) {
	response, body, err := sdk.Get("site", path, query, headers, sdk.ResponseExpect{ContentTypes: expectContentTypes(headers["Accept"]), MaxBytes: maxBytes})
	if err != nil {
		return sdk.HostResponse{}, nil, err
	}
	if response.Status < 200 || response.Status >= 300 {
		return response, nil, fmt.Errorf("provider request %s failed (%d): %s", path, response.Status, string(body))
	}
	return response, body, nil
}

func getProviderURL(rawURL string, contentTypes []string, maxBytes int64) ([]byte, error) {
	return getProviderURLWithCookie(rawURL, "", contentTypes, maxBytes)
}

func getProviderURLWithCookie(rawURL string, cookie string, contentTypes []string, maxBytes int64) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	connector := "site"
	if parsed.IsAbs() {
		switch parsed.Host {
		case "www.schweizer-wanderwege.ch":
			connector = "site"
		case "prod.sww-geo.apps.gs-ch-prod.camptocamp.com":
			connector = "geo"
			cookie = ""
		default:
			return nil, fmt.Errorf("unsupported provider file host %q", parsed.Host)
		}
	}
	path := parsed.EscapedPath()
	query := make([]sdk.QueryParam, 0)
	for name, values := range parsed.Query() {
		for _, value := range values {
			query = append(query, sdk.QueryParam{Name: name, Value: value})
		}
	}
	_, body, err := connectorRequest(connector, "GET", path, query, headersWithCookie(contentTypes, cookie), nil, contentTypes, maxBytes, true)
	if err != nil {
		return nil, fmt.Errorf("provider file request %s via %s failed: %w", rawURL, connector, err)
	}
	return body, err
}

func extractHikeIDs(body []byte) ([]string, error) {
	var payload mapFilterResponse
	if err := json.Unmarshal(body, &payload); err == nil && len(payload.Hikes) > 0 {
		return payload.Hikes, nil
	}

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	ids := findHikeIDs(raw)
	if len(ids) == 0 {
		return nil, fmt.Errorf("map filter response did not contain hikes")
	}
	return ids, nil
}

func findHikeIDs(value any) []string {
	switch typed := value.(type) {
	case map[string]any:
		if hikes, ok := typed["hikes"].([]any); ok {
			return idsFromRawList(hikes)
		}
		for _, child := range typed {
			if ids := findHikeIDs(child); len(ids) > 0 {
				return ids
			}
		}
	case []any:
		for _, child := range typed {
			if ids := findHikeIDs(child); len(ids) > 0 {
				return ids
			}
		}
	}
	return nil
}

func idsFromRawList(items []any) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			ids = append(ids, typed)
		case map[string]any:
			if uuid, ok := typed["uuid"].(string); ok && uuid != "" {
				ids = append(ids, uuid)
			} else if id, ok := typed["id"].(string); ok && id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func acceptHeader(contentTypes []string) string {
	if len(contentTypes) == 0 {
		return "*/*"
	}
	value := ""
	for i, contentType := range contentTypes {
		if i > 0 {
			value += ","
		}
		value += contentType
	}
	return value
}

func baseHeaders(contentTypes []string) map[string]string {
	return map[string]string{
		"Accept":     acceptHeader(contentTypes),
		"User-Agent": "wanderer-swiss-hike-plugin/0.1",
	}
}

func headersWithCookie(contentTypes []string, cookie string) map[string]string {
	headers := baseHeaders(contentTypes)
	if cookie != "" {
		headers["Cookie"] = cookie
	}
	return headers
}

func cookieHeader(response sdk.HostResponse) string {
	values := response.HeaderValuesFor("Set-Cookie")
	if len(values) == 0 {
		if value := response.FirstHeader("Set-Cookie"); value != "" {
			values = []string{value}
		}
	}
	if len(values) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(values))
	for _, value := range values {
		pairs = append(pairs, cookiePair(value))
	}
	return strings.Join(pairs, "; ")
}

func cookiePair(value string) string {
	if index := strings.Index(value, ";"); index >= 0 {
		return value[:index]
	}
	return value
}

func expectContentTypes(accept string) []string {
	if accept == "" || accept == "*/*" {
		return nil
	}
	parts := strings.Split(accept, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

type requestContext struct {
	debugLogging bool
}

func (ctx requestContext) logElapsed(label string, start time.Time, detail string) {
	ctx.logDebug(label, "took "+time.Since(start).String()+detailSuffix(detail))
}

func (ctx requestContext) logDebug(label string, detail string) {
	if !ctx.debugLogging {
		return
	}
	message := "swiss-hike " + label
	if detail != "" {
		message += " " + detail
	}
	sdk.LogDebug(message)
}

func detailSuffix(detail string) string {
	if detail == "" {
		return ""
	}
	return " " + detail
}
