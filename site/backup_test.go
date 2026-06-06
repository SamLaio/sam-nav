package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportCardsBackupRestoresBoxesAndCards(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	backup := backupImportFile{
		Scope: "cards",
		Categories: []categoryBox{
			{Name: "工具", SortOrder: 1},
			{Name: "文件", SortOrder: 2},
		},
		Links: []linkCard{
			{Title: "Tool", URL: "https://tool.example.com", Category: "工具", SortOrder: 1},
			{Title: "Doc", URL: "https://doc.example.com", Category: "文件", SortOrder: 2, Hidden: true},
		},
	}
	if _, err := importBackup(ctx, db, "cards", backup, text); err != nil {
		t.Fatalf("importBackup 發生錯誤：%v", err)
	}

	categories, err := listCategories(ctx, db)
	if err != nil {
		t.Fatalf("listCategories 發生錯誤：%v", err)
	}
	gotCategories := []string{categories[0].Name, categories[1].Name, categories[2].Name}
	wantCategories := []string{"工具", "文件", defaultCategoryName}
	for index := range wantCategories {
		if gotCategories[index] != wantCategories[index] {
			t.Fatalf("卡片盒 = %v，預期 %v", gotCategories, wantCategories)
		}
	}

	links, err := listAdminLinks(ctx, db)
	if err != nil {
		t.Fatalf("listAdminLinks 發生錯誤：%v", err)
	}
	if len(links) != 2 || links[0].Category != "工具" || links[1].Category != "文件" {
		t.Fatalf("匯入卡片結果不正確：%v", links)
	}
}

func TestImportSettingsBackup(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	settings, err := importBackup(ctx, db, "settings", backupImportFile{
		Scope: "settings",
		Settings: &appSettings{
			SiteTitle:       "Imported",
			Language:        "en",
			DefaultTheme:    "dark",
			OpenNewTab:      true,
			Background:      "https://example.com/bg.jpg",
			SearchEngineURL: "https://www.google.com/search?q=%s",
			AdminUsername:   "owner",
		},
	}, text)
	if err != nil {
		t.Fatalf("importBackup 發生錯誤：%v", err)
	}
	if settings.SiteTitle != "Imported" || settings.Language != "en" || settings.DefaultTheme != "dark" {
		t.Fatalf("系統設定匯入結果不正確：%+v", settings)
	}
}

func TestSettingsBackupIncludesSearchEngines(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	backup, err := exportBackup(ctx, db, "settings", mustReadLanguage(defaultLanguageName).text)
	if err != nil {
		t.Fatalf("exportBackup 發生錯誤：%v", err)
	}
	if len(backup.SearchEngines) != 1 || backup.SearchEngines[0].Name != "Google" {
		t.Fatalf("系統設定備份應包含 Google 搜尋引擎，取得：%+v", backup.SearchEngines)
	}

	text := mustReadLanguage(defaultLanguageName).text
	if _, err := importBackup(ctx, db, "settings", backupImportFile{
		Scope: "settings",
		Settings: &appSettings{
			SiteTitle:     "Imported",
			Language:      defaultLanguageName,
			DefaultTheme:  "light",
			AdminUsername: "admin",
		},
		SearchEngines: []searchEngine{
			{Name: "Docs", URL: "https://docs.example.com/search?q=%s", Enabled: true},
		},
	}, text); err != nil {
		t.Fatalf("importBackup 發生錯誤：%v", err)
	}
	engines, err := listSearchEngines(ctx, db)
	if err != nil {
		t.Fatalf("listSearchEngines 發生錯誤：%v", err)
	}
	if len(engines) != 1 || engines[0].Name != "Docs" {
		t.Fatalf("匯入後搜尋引擎不正確：%+v", engines)
	}
}

func TestReadImportBackupSupportsLegacyNavFormat(t *testing.T) {
	text := mustReadLanguage(defaultLanguageName).text
	backup, err := readImportBackup(strings.NewReader(`[
		{
			"id": 15,
			"name": "Router",
			"url": "https://router.example.com/",
			"logo": "https://example.com/icon.png",
			"catelog": "MyHome",
			"desc": "Router",
			"sort": 1,
			"hide": true
		}
	]`), text)
	if err != nil {
		t.Fatalf("readImportBackup 發生錯誤：%v", err)
	}
	if len(backup.Links) != 1 {
		t.Fatalf("應匯入 1 張卡片，取得：%d", len(backup.Links))
	}
	link := backup.Links[0]
	if link.Title != "Router" || link.Category != "MyHome" || link.Description != "Router" || link.Icon == "" {
		t.Fatalf("舊格式欄位未正確對應：%+v", link)
	}
	if link.SortOrder != 1 || !link.Hidden {
		t.Fatalf("sort/hide 未正確對應：%+v", link)
	}
}

func TestImportRejectsMismatchedScope(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	_, err = importBackup(context.Background(), db, "cards", backupImportFile{
		Scope: "settings",
		Settings: &appSettings{
			SiteTitle:     "Imported",
			Language:      defaultLanguageName,
			DefaultTheme:  "light",
			AdminUsername: "admin",
		},
	}, mustReadLanguage(defaultLanguageName).text)
	if err == nil {
		t.Fatal("不同範圍的備份檔不應匯入")
	}
}

func TestImportCardsBackupNormalizesIconURL(t *testing.T) {
	withTestGoogleFaviconClient(t)
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	server := newTestIconServer(t)
	text := mustReadLanguage(defaultLanguageName).text
	if _, err := importBackup(context.Background(), db, "cards", backupImportFile{
		Scope: "cards",
		Links: []linkCard{
			{Title: "Page", URL: "https://page.example.com", Icon: server.URL + "/page.html"},
			{Title: "Image", URL: "https://image.example.com", Icon: server.URL + "/image.png"},
		},
	}, text); err != nil {
		t.Fatalf("匯入圖示應完成正規化，取得：%v", err)
	}
	links, err := listAdminLinks(context.Background(), db)
	if err != nil {
		t.Fatalf("listAdminLinks 發生錯誤：%v", err)
	}
	if len(links) != 2 {
		t.Fatalf("匯入卡片數量 = %d，預期 2", len(links))
	}
	if links[0].Icon != "https://www.google.com/s2/favicons?domain=page.example.com&sz=64" {
		t.Fatalf("壞圖示應改補 favicon，取得：%s", links[0].Icon)
	}
	if links[1].Icon != server.URL+"/image.png" {
		t.Fatalf("有效圖示應保留，取得：%s", links[1].Icon)
	}
}

func TestExportCardsBackupNormalizesIconURL(t *testing.T) {
	withTestGoogleFaviconClient(t)
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	server := newTestIconServer(t)
	text := mustReadLanguage(defaultLanguageName).text
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO nav_links (title, url, description, category, icon, sort_order, hidden, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`,
		"Broken Icon",
		"https://broken.example.com",
		"",
		defaultCategoryName,
		server.URL+"/page.html",
		1,
		false,
	); err != nil {
		t.Fatalf("建立測試卡片失敗：%v", err)
	}

	backup, err := exportBackup(ctx, db, "cards", text)
	if err != nil {
		t.Fatalf("匯出卡片應完成圖示正規化，取得：%v", err)
	}
	if len(backup.Links) != 1 {
		t.Fatalf("匯出卡片數量 = %d，預期 1", len(backup.Links))
	}
	if backup.Links[0].Icon != "https://www.google.com/s2/favicons?domain=broken.example.com&sz=64" {
		t.Fatalf("匯出壞圖示應改補 favicon，取得：%s", backup.Links[0].Icon)
	}
}

func newTestIconServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image.png" {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html></html>"))
	}))
}
