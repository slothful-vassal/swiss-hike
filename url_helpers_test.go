package main

import "testing"

func TestNormalizeLocationURLPreservesOAuthParameterNames(t *testing.T) {
	raw := "https://rando-community.ch/oauth/authorize?client_id=abc&response_type=code&redirect_uri=https%3A%2F%2Fwww.schweizer-wanderwege.ch%2Flogin_check&scope=openid+email"

	if got := normalizeLocationURL(raw); got != raw {
		t.Fatalf("normalizeLocationURL() = %q, want %q", got, raw)
	}
}

func TestNormalizeLocationURLDecodesEscapedAmpersands(t *testing.T) {
	raw := "https://rando-community.ch/oauth/authorize?client_id=abc&amp;response_type=code&amp;scope=openid+email"
	want := "https://rando-community.ch/oauth/authorize?client_id=abc&response_type=code&scope=openid+email"

	if got := normalizeLocationURL(raw); got != want {
		t.Fatalf("normalizeLocationURL() = %q, want %q", got, want)
	}
}
