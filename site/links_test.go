package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

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

func TestNormalizeLinkIconKeepsValidImageAndFallsBack(t *testing.T) {
	withTestGoogleFaviconClient(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pngHeader := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
		switch r.URL.Path {
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(pngHeader)
		case "/page-with-icon":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/image.png"></head></html>`))
		case "/page-with-base":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><head><base href="/assets/"><link rel="apple-touch-icon" href="../image.png"></head></html>`))
		case "/page-with-data-icon":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1 1'%3E%3C/svg%3E"></head></html>`))
		case "/get-only.png":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(pngHeader)
		default:
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html></html>"))
		}
	}))
	defer server.Close()

	tests := []struct {
		name     string
		url      string
		icon     string
		expected string
	}{
		{name: "空白先讀頁面 icon", url: server.URL + "/page-with-icon", icon: "", expected: server.URL + "/image.png"},
		{name: "空白支援 base 與 apple touch icon", url: server.URL + "/page-with-base", icon: "", expected: server.URL + "/image.png"},
		{name: "空白支援 HTML 內嵌圖示", url: server.URL + "/page-with-data-icon", icon: "", expected: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1 1'%3E%3C/svg%3E"},
		{name: "找不到頁面 icon 時補預設 favicon", url: server.URL + "/empty-page", icon: "", expected: testGeneratedFaviconURL(server.URL + "/empty-page")},
		{name: "圖片網址保留", url: "https://example.com/docs", icon: server.URL + "/image.png", expected: server.URL + "/image.png"},
		{name: "HEAD 失敗時改用 GET", url: "https://example.com/docs", icon: server.URL + "/get-only.png", expected: server.URL + "/get-only.png"},
		{name: "非圖片網址改讀頁面 icon", url: server.URL + "/page-with-icon", icon: server.URL + "/page.html", expected: server.URL + "/image.png"},
		{name: "非 HTTP 網址改補 favicon", url: server.URL + "/empty-page", icon: "mailto:admin@example.com", expected: testGeneratedFaviconURL(server.URL + "/empty-page")},
		{name: "系統 favicon 依目前網址重算", url: server.URL + "/empty-page", icon: "https://www.google.com/s2/favicons?domain=old.example.com&sz=64", expected: testGeneratedFaviconURL(server.URL + "/empty-page")},
		{name: "自動 favicon 抓不到時留空", url: "https://no-icon.example.com", icon: "", expected: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			card := normalizeLinkIcon(context.Background(), linkCard{
				URL:  test.url,
				Icon: test.icon,
			})
			if card.Icon != test.expected {
				t.Fatalf("Icon = %q，預期 %q", card.Icon, test.expected)
			}
		})
	}
}

func TestLinkIconURLHasImageAllowsSelfSignedHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	}))
	defer server.Close()

	if !linkIconURLHasImage(context.Background(), server.URL+"/icon.png") {
		t.Fatal("自簽 HTTPS 圖示應視為有效圖片")
	}
}

func TestRefreshLinkIconCacheWritesDataImageIcon(t *testing.T) {
	cacheDir := t.TempDir()
	card := refreshLinkIconCache(context.Background(), cacheDir, linkCard{
		ID:   1,
		Icon: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1 1'%3E%3C/svg%3E",
	}, true)
	if card.Icon == "" {
		t.Fatal("內嵌圖示應保留來源並寫入快取")
	}
	content, contentType, ok := readCachedLinkIcon(cacheDir, 1)
	if !ok {
		t.Fatal("內嵌圖示應另存為快取檔")
	}
	if contentType != "image/svg+xml" || !strings.Contains(string(content), "<svg") {
		t.Fatalf("快取內容應為 SVG 圖片，contentType=%q content=%q", contentType, string(content))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func withTestGoogleFaviconClient(t *testing.T) {
	t.Helper()
	previousClient := linkIconHTTPClient
	baseTransport := http.DefaultTransport
	linkIconHTTPClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host == "no-icon.example.com" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header: http.Header{
						"Content-Type": []string{"text/html"},
					},
					Body:    io.NopCloser(strings.NewReader("<html></html>")),
					Request: request,
				}, nil
			}
			if request.URL.Host != "www.google.com" {
				return baseTransport.RoundTrip(request)
			}
			domain := request.URL.Query().Get("domain")
			contentType := "image/png"
			content := string([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
			if domain == "no-icon.example.com" {
				contentType = "text/html"
				content = "<html></html>"
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{contentType},
				},
				Body:    io.NopCloser(strings.NewReader(content)),
				Request: request,
			}, nil
		}),
		Timeout: iconValidationTimeout,
	}
	t.Cleanup(func() {
		linkIconHTTPClient = previousClient
	})
}

func testGeneratedFaviconURL(rawURL string) string {
	parsed, _ := url.Parse(rawURL)
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(parsed.Hostname()) + "&sz=64"
}
