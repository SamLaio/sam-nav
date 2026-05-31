package main

import "testing"

func TestFaviconURL(t *testing.T) {
	tests := map[string]string{
		"https://example.com/docs":        "https://www.google.com/s2/favicons?domain=example.com&sz=64",
		"http://example.com:8080/path":    "https://www.google.com/s2/favicons?domain=example.com&sz=64",
		"mailto:admin@example.com":        "",
		"not a url":                       "",
		"https://example.com/favicon.ico": "https://www.google.com/s2/favicons?domain=example.com&sz=64",
	}

	for rawURL, expected := range tests {
		if got := faviconURL(rawURL); got != expected {
			t.Fatalf("faviconURL(%q) = %q，預期 %q", rawURL, got, expected)
		}
	}
}

func TestCleanLinkReplacesClearbitIcon(t *testing.T) {
	card := cleanLink(linkCard{
		Title: "Kobo",
		URL:   "https://www.kobo.com/tw/zh",
		Icon:  "https://logo.clearbit.com/www.kobo.com?size=320",
	})
	if card.Icon != "https://www.google.com/s2/favicons?domain=www.kobo.com&sz=64" {
		t.Fatalf("Clearbit 圖示應改成 Google favicon，取得：%s", card.Icon)
	}
}
