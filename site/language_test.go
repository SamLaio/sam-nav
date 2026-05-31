package main

import "testing"

func TestLanguageFilesUseSameKeys(t *testing.T) {
	defaultLanguage := mustReadLanguage(defaultLanguageName)
	english := mustReadLanguage("en")

	for key := range defaultLanguage {
		if english[key] == "" {
			t.Fatalf("en.json 缺少語系 key：%s", key)
		}
	}
	for key := range english {
		if defaultLanguage[key] == "" {
			t.Fatalf("zhTW.json 缺少語系 key：%s", key)
		}
	}
}

func TestLoadLanguage(t *testing.T) {
	if got := loadLanguage("en").text("html_lang"); got != "en" {
		t.Fatalf("載入英文語系失敗，取得：%s", got)
	}

	if got := loadLanguage("missing").text("html_lang"); got != "zhTW" {
		t.Fatalf("不存在的語系應 fallback 到 zhTW，取得：%s", got)
	}
}

func TestNormalizeLanguageName(t *testing.T) {
	tests := map[string]string{
		"zhTW":    "zhTW",
		"en":      "en",
		"missing": "",
		"zh":      "",
	}

	for raw, expected := range tests {
		if got := normalizeLanguageName(raw); got != expected {
			t.Fatalf("normalizeLanguageName(%q) = %q，預期 %q", raw, got, expected)
		}
	}
}
