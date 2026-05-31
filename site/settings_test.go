package main

import (
	"context"
	"testing"
)

func TestUpdateAppSettingsLanguage(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	settings, err := updateAppSettings(ctx, db, settingsPayload{
		SiteTitle:       "Sam Nav",
		Language:        "en",
		DefaultTheme:    "dark",
		OpenNewTab:      true,
		SearchEngineURL: "https://www.google.com/search?q=%s",
		AdminUsername:   "admin",
	}, text)
	if err != nil {
		t.Fatalf("updateAppSettings 發生錯誤：%v", err)
	}
	if settings.Language != "en" {
		t.Fatalf("系統語言應更新為 en，取得：%s", settings.Language)
	}

	loaded, err := loadAppSettings(ctx, db)
	if err != nil {
		t.Fatalf("loadAppSettings 發生錯誤：%v", err)
	}
	if loaded.Language != "en" {
		t.Fatalf("讀回系統語言應為 en，取得：%s", loaded.Language)
	}
}

func TestUpdateAppSettingsRejectsInvalidLanguage(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	_, err = updateAppSettings(context.Background(), db, settingsPayload{
		SiteTitle:     "Sam Nav",
		Language:      "missing",
		DefaultTheme:  "light",
		AdminUsername: "admin",
	}, mustReadLanguage(defaultLanguageName).text)
	if err == nil {
		t.Fatal("不支援的系統語言應回傳錯誤")
	}
}
