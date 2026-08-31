package main

import (
	"context"
	"testing"
)

func TestSearchHistoryLifecycle(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	if err := recordSearchHistory(ctx, db, "test", 2, text); err != nil {
		t.Fatalf("recordSearchHistory 發生錯誤：%v", err)
	}
	if err := recordSearchHistory(ctx, db, "test", 2, text); err != nil {
		t.Fatalf("recordSearchHistory 重複關鍵字發生錯誤：%v", err)
	}
	matches, err := listSearchHistory(ctx, db, "test", 10)
	if err != nil {
		t.Fatalf("listSearchHistory 搜尋發生錯誤：%v", err)
	}
	if len(matches) != 1 || matches[0].Keyword != "test" || matches[0].HitCount != 2 {
		t.Fatalf("搜尋歷史去重或計數不正確：%+v", matches)
	}

	if err := deleteSearchHistory(ctx, db, []int64{matches[0].ID}, false, text); err != nil {
		t.Fatalf("deleteSearchHistory 單筆刪除發生錯誤：%v", err)
	}
	if err := recordSearchHistory(ctx, db, "speed test", 2, text); err != nil {
		t.Fatalf("recordSearchHistory 發生錯誤：%v", err)
	}
	if err := recordSearchHistory(ctx, db, "third", 2, text); err != nil {
		t.Fatalf("recordSearchHistory 發生錯誤：%v", err)
	}
	if err := recordSearchHistory(ctx, db, "fourth", 2, text); err != nil {
		t.Fatalf("recordSearchHistory 修剪上限發生錯誤：%v", err)
	}

	items, err := listSearchHistory(ctx, db, "", 10)
	if err != nil {
		t.Fatalf("listSearchHistory 發生錯誤：%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("搜尋歷史應保留 2 筆，取得：%d", len(items))
	}

	if err := deleteSearchHistory(ctx, db, nil, true, text); err != nil {
		t.Fatalf("deleteSearchHistory 全部刪除發生錯誤：%v", err)
	}
	items, err = listSearchHistory(ctx, db, "", 10)
	if err != nil {
		t.Fatalf("listSearchHistory 刪除後發生錯誤：%v", err)
	}
	if len(items) != 0 {
		t.Fatalf("搜尋歷史應已清空，取得：%+v", items)
	}
}
