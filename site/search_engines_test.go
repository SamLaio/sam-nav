package main

import (
	"context"
	"testing"
)

func TestDefaultSearchEngineIsGoogle(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	engines, err := listVisibleSearchEngines(context.Background(), db)
	if err != nil {
		t.Fatalf("listVisibleSearchEngines 發生錯誤：%v", err)
	}
	if len(engines) != 1 {
		t.Fatalf("應有 1 個預設搜尋引擎，取得：%d", len(engines))
	}
	if engines[0].Name != "Google" || engines[0].URL != defaultSearchEngineURL || !engines[0].Enabled {
		t.Fatalf("預設搜尋引擎不正確：%+v", engines[0])
	}
}

func TestSearchEngineLifecycleAndSort(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	if _, err := db.ExecContext(ctx, `DELETE FROM nav_search_engines;`); err != nil {
		t.Fatalf("清空預設搜尋引擎發生錯誤：%v", err)
	}
	first, err := createSearchEngine(ctx, db, searchEngine{
		Name:    "Docs",
		URL:     "https://docs.example.com/search?q=%s",
		Enabled: true,
	}, text)
	if err != nil {
		t.Fatalf("createSearchEngine 發生錯誤：%v", err)
	}
	second, err := createSearchEngine(ctx, db, searchEngine{
		Name:    "Code",
		URL:     "https://code.example.com/search?q=%s",
		Enabled: true,
	}, text)
	if err != nil {
		t.Fatalf("createSearchEngine 發生錯誤：%v", err)
	}

	if err := updateSearchEnginesSort(ctx, db, []searchEngineSortUpdate{
		{ID: second.ID, SortOrder: 1},
		{ID: first.ID, SortOrder: 2},
	}, text); err != nil {
		t.Fatalf("updateSearchEnginesSort 發生錯誤：%v", err)
	}
	engines, err := listSearchEngines(ctx, db)
	if err != nil {
		t.Fatalf("listSearchEngines 發生錯誤：%v", err)
	}
	if engines[0].ID != second.ID {
		t.Fatalf("排序第一筆應為 Code，取得：%+v", engines[0])
	}

	second.Enabled = false
	if _, err := updateSearchEngine(ctx, db, second, text); err != nil {
		t.Fatalf("updateSearchEngine 發生錯誤：%v", err)
	}
	visible, err := listVisibleSearchEngines(ctx, db)
	if err != nil {
		t.Fatalf("listVisibleSearchEngines 發生錯誤：%v", err)
	}
	for _, engine := range visible {
		if engine.ID == second.ID {
			t.Fatalf("停用的搜尋引擎不應出現在前台：%+v", visible)
		}
	}
}
