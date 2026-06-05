//go:build tinygo

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/open-wanderer/wanderer/plugins/sdk"
)

const (
	randoConnector  = "community"
	loginMaxBytes   = 1024 * 1024
	sessionCacheTTL = 15 * time.Minute
)

type providerSession struct {
	siteCookie  string
	randoCookie string
}

type cachedProviderSession struct {
	key       string
	expiresAt time.Time
	session   *providerSession
}

var cachedSession *cachedProviderSession

func loginSessionCached(ctx requestContext, auth map[string]any) (*providerSession, bool, error) {
	email := sdk.StringField(auth, "email")
	password := sdk.StringField(auth, "password")
	if email == "" || password == "" {
		return nil, false, fmt.Errorf("email and password are required")
	}
	key := sessionCacheKey(email, password)
	if cachedSession != nil &&
		cachedSession.key == key &&
		cachedSession.session != nil &&
		cachedSession.session.siteCookie != "" &&
		time.Now().Before(cachedSession.expiresAt) {
		return cachedSession.session, true, nil
	}

	session, err := loginSessionWithCredentials(ctx, email, password)
	if err != nil {
		return nil, false, err
	}
	cachedSession = &cachedProviderSession{
		key:       key,
		expiresAt: time.Now().Add(sessionCacheTTL),
		session:   session,
	}
	return session, false, nil
}

func sessionCacheKey(email string, password string) string {
	sum := sha256.Sum256([]byte(email + "\x00" + password))
	return hex.EncodeToString(sum[:])
}

func loginSession(ctx requestContext, auth map[string]any) (*providerSession, error) {
	email := sdk.StringField(auth, "email")
	password := sdk.StringField(auth, "password")
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}
	return loginSessionWithCredentials(ctx, email, password)
}

func loginSessionWithCredentials(ctx requestContext, email string, password string) (*providerSession, error) {
	session := &providerSession{}
	start := time.Now()
	location, err := session.startSiteLogin()
	if err != nil {
		return nil, err
	}
	ctx.logElapsed("login start site", start, "location="+locationForLog(location))
	if location == "" {
		return nil, fmt.Errorf("site login did not return oauth location")
	}

	start = time.Now()
	location, err = session.getRedirect(location, "site")
	if err != nil {
		return nil, err
	}
	ctx.logElapsed("login site redirect", start, "location="+locationForLog(location))
	start = time.Now()
	loginBody, err := session.getBody(location, []string{"text/html"})
	if err != nil {
		return nil, err
	}
	ctx.logElapsed("login rando form", start, fmt.Sprintf("bytes=%d", len(loginBody)))
	token := hiddenAuthenticityToken(loginBody)
	if token == "" {
		return nil, fmt.Errorf("could not find rando login authenticity token")
	}

	start = time.Now()
	location, err = session.postRandoLogin(email, password, token)
	if err != nil {
		return nil, err
	}
	ctx.logElapsed("login rando post", start, "location="+locationForLog(location))
	if location == "" {
		return nil, fmt.Errorf("rando login did not return redirect; credentials may be invalid")
	}
	start = time.Now()
	if err := session.followLoginRedirects(location); err != nil {
		return nil, err
	}
	ctx.logElapsed("login follow redirects", start, "")
	return session, nil
}

func (s *providerSession) startSiteLogin() (string, error) {
	response, _, err := connectorRequest("site", "GET", "/de/login", nil, headersWithCookie([]string{"text/html"}, s.siteCookie), nil, []string{"text/html"}, loginMaxBytes, false)
	if err != nil {
		return "", err
	}
	s.siteCookie = mergeCookies(s.siteCookie, response)
	return responseLocation(response), nil
}

func (s *providerSession) getRedirect(rawLocation string, defaultConnector string) (string, error) {
	connector, path, query, err := connectorTarget(rawLocation, defaultConnector)
	if err != nil {
		return "", err
	}
	response, _, err := connectorRequest(connector, "GET", path, query, s.headers(connector, []string{"text/html"}), nil, []string{"text/html"}, loginMaxBytes, false)
	if err != nil {
		return "", err
	}
	s.mergeConnectorCookies(connector, response)
	return resolveConnectorLocation(connector, responseLocation(response)), nil
}

func (s *providerSession) getBody(rawLocation string, contentTypes []string) ([]byte, error) {
	connector, path, query, err := connectorTarget(rawLocation, "site")
	if err != nil {
		return nil, err
	}
	response, body, err := connectorRequest(connector, "GET", path, query, s.headers(connector, contentTypes), nil, contentTypes, loginMaxBytes, false)
	if err != nil {
		return nil, err
	}
	s.mergeConnectorCookies(connector, response)
	if response.Status < 200 || response.Status >= 300 {
		return nil, fmt.Errorf("%s returned status %d", locationForLog(rawLocation), response.Status)
	}
	return body, nil
}

func (s *providerSession) postRandoLogin(email string, password string, token string) (string, error) {
	headers := s.headers(randoConnector, []string{"text/html"})
	headers["Referer"] = "https://rando-community.ch/de/users/sign_in?oauth=true"
	response, body, err := connectorRequest(randoConnector, "POST", "/de/users/sign_in", []sdk.QueryParam{{Name: "oauth", Value: "true"}}, headers, &sdk.HostRequestBody{
		Type: sdk.HostRequestBodyTypeForm,
		Form: []sdk.FormField{
			{Name: "authenticity_token", Value: token},
			{Name: "person[login_identity]", Value: email},
			{Name: "person[password]", Value: password},
			{Name: "person[remember_me]", Value: "1"},
		},
	}, []string{"text/html"}, loginMaxBytes, false)
	if err != nil {
		return "", err
	}
	s.mergeConnectorCookies(randoConnector, response)
	if response.Status >= 200 && response.Status < 300 {
		if strings.Contains(string(body), "is-invalid") || strings.Contains(string(body), "alert") {
			return "", fmt.Errorf("rando login returned form page; credentials may be invalid")
		}
	}
	return resolveConnectorLocation(randoConnector, responseLocation(response)), nil
}

func (s *providerSession) followLoginRedirects(location string) error {
	for i := 0; i < 12 && location != ""; i++ {
		connector, path, query, err := connectorTarget(location, "site")
		if err != nil {
			return err
		}
		if connector == "site" && path == "/" {
			return nil
		}
		response, _, err := connectorRequest(connector, "GET", path, query, s.headers(connector, []string{"text/html"}), nil, []string{"text/html"}, loginMaxBytes, false)
		if err != nil {
			return err
		}
		s.mergeConnectorCookies(connector, response)
		if connector == "site" && response.Status >= 200 && response.Status < 300 {
			return nil
		}
		location = resolveConnectorLocation(connector, responseLocation(response))
		if location == "" && response.Status >= 200 && response.Status < 300 {
			return fmt.Errorf("login flow stopped on %s before returning to Schweizer Wanderwege", connector)
		}
	}
	return fmt.Errorf("login redirect flow did not finish")
}

func (s *providerSession) headers(connector string, contentTypes []string) map[string]string {
	cookie := s.siteCookie
	if connector == randoConnector {
		cookie = s.randoCookie
	}
	return headersWithCookie(contentTypes, cookie)
}

func (s *providerSession) mergeConnectorCookies(connector string, response sdk.HostResponse) {
	if connector == randoConnector {
		s.randoCookie = mergeCookies(s.randoCookie, response)
		return
	}
	s.siteCookie = mergeCookies(s.siteCookie, response)
}

func connectorRequest(connector string, method string, path string, query []sdk.QueryParam, headers map[string]string, body *sdk.HostRequestBody, contentTypes []string, maxBytes int64, followRedirects bool) (sdk.HostResponse, []byte, error) {
	response, responseBody, err := sdk.HostRequest(sdk.HostRequestSpec{
		Method: method,
		Target: sdk.RequestTarget{
			Type:      "connector",
			Connector: connector,
			Path:      path,
			Query:     query,
		},
		Headers:         headers,
		Body:            body,
		Expect:          sdk.ResponseExpect{ContentTypes: contentTypes, MaxBytes: maxBytes},
		FollowRedirects: sdk.Bool(followRedirects),
	})
	if err != nil {
		return response, responseBody, fmt.Errorf("connector request %s %s %s failed: %w", method, connector, path, err)
	}
	if response.Status >= 400 {
		return response, responseBody, fmt.Errorf("%s %s failed (%d): %s", method, path, response.Status, string(responseBody))
	}
	return response, responseBody, nil
}

func connectorTarget(rawLocation string, defaultConnector string) (string, string, []sdk.QueryParam, error) {
	parsed, err := url.Parse(htmlUnescape(rawLocation))
	if err != nil {
		return "", "", nil, err
	}
	connector := defaultConnector
	if connector == "" {
		connector = "site"
	}
	if parsed.IsAbs() {
		switch parsed.Host {
		case "www.schweizer-wanderwege.ch":
			connector = "site"
		case "rando-community.ch":
			connector = randoConnector
		default:
			return "", "", nil, fmt.Errorf("unsupported login redirect host %q", parsed.Host)
		}
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	query := make([]sdk.QueryParam, 0)
	for name, values := range parsed.Query() {
		for _, value := range values {
			query = append(query, sdk.QueryParam{Name: name, Value: value})
		}
	}
	return connector, path, query, nil
}

func resolveConnectorLocation(connector string, location string) string {
	if location == "" {
		return ""
	}
	parsed, err := url.Parse(htmlUnescape(location))
	if err != nil || parsed.IsAbs() {
		return location
	}
	if connector == randoConnector {
		return "https://rando-community.ch" + location
	}
	return "https://www.schweizer-wanderwege.ch" + location
}

func hiddenAuthenticityToken(body []byte) string {
	return inputValue(string(body), "authenticity_token")
}

func responseLocation(response sdk.HostResponse) string {
	return response.FirstHeader("Location")
}

func locationForLog(rawLocation string) string {
	parsed, err := url.Parse(rawLocation)
	if err != nil {
		return "<invalid>"
	}
	if parsed.Path == "" {
		return "/"
	}
	return parsed.EscapedPath()
}

func mergeCookies(current string, response sdk.HostResponse) string {
	values := response.HeaderValuesFor("Set-Cookie")
	if len(values) == 0 {
		if value := response.FirstHeader("Set-Cookie"); value != "" {
			values = []string{value}
		}
	}
	if len(values) == 0 {
		return current
	}
	cookies := map[string]string{}
	order := make([]string, 0)
	for _, pair := range strings.Split(current, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name := cookieName(pair)
		if name == "" {
			continue
		}
		if _, ok := cookies[name]; !ok {
			order = append(order, name)
		}
		cookies[name] = pair
	}
	for _, value := range values {
		pair := cookiePair(value)
		name := cookieName(pair)
		if name == "" {
			continue
		}
		if _, ok := cookies[name]; !ok {
			order = append(order, name)
		}
		cookies[name] = pair
	}
	result := make([]string, 0, len(order))
	for _, name := range order {
		result = append(result, cookies[name])
	}
	return strings.Join(result, "; ")
}

func cookieName(pair string) string {
	if index := strings.Index(pair, "="); index > 0 {
		return pair[:index]
	}
	return ""
}
