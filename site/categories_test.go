package main

import (
	"context"
	"strings"
	"testing"
)

func TestCategoryLifecycleMovesCardsToDefault(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text

	if err := createCategory(ctx, db, "工具", text); err != nil {
		t.Fatalf("createCategory 發生錯誤：%v", err)
	}
	if _, err := createLink(ctx, db, linkCard{
		Title:    "Example",
		URL:      "https://example.com",
		Category: "工具",
	}, text); err != nil {
		t.Fatalf("createLink 發生錯誤：%v", err)
	}
	if err := renameCategory(ctx, db, "工具", "文件", text); err != nil {
		t.Fatalf("renameCategory 發生錯誤：%v", err)
	}

	links, err := listAdminLinks(ctx, db)
	if err != nil {
		t.Fatalf("listAdminLinks 發生錯誤：%v", err)
	}
	if links[0].Category != "文件" {
		t.Fatalf("改名後卡片應移到文件，取得：%s", links[0].Category)
	}

	if err := deleteCategory(ctx, db, "文件", text); err != nil {
		t.Fatalf("deleteCategory 發生錯誤：%v", err)
	}
	links, err = listAdminLinks(ctx, db)
	if err != nil {
		t.Fatalf("listAdminLinks 發生錯誤：%v", err)
	}
	if links[0].Category != defaultCategoryName {
		t.Fatalf("刪除卡片盒後卡片應移到 %s，取得：%s", defaultCategoryName, links[0].Category)
	}
}

func TestDefaultCategorySortedLast(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	for _, name := range []string{"工具", "文件"} {
		if err := createCategory(ctx, db, name, text); err != nil {
			t.Fatalf("createCategory(%s) 發生錯誤：%v", name, err)
		}
	}

	categories, err := listCategories(ctx, db)
	if err != nil {
		t.Fatalf("listCategories 發生錯誤：%v", err)
	}
	if got := categories[len(categories)-1].Name; got != defaultCategoryName {
		t.Fatalf("%s 應排在最後，最後一筆為：%s", defaultCategoryName, got)
	}
}

func TestListVisibleCategoriesOnlyShowsVisibleCardBoxes(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	for _, name := range []string{"工具", "文件"} {
		if err := createCategory(ctx, db, name, text); err != nil {
			t.Fatalf("createCategory(%s) 發生錯誤：%v", name, err)
		}
	}
	if _, err := createLink(ctx, db, linkCard{
		Title:    "Visible Tool",
		URL:      "https://tool.example.com",
		Category: "工具",
	}, text); err != nil {
		t.Fatalf("createLink(工具) 發生錯誤：%v", err)
	}
	if _, err := createLink(ctx, db, linkCard{
		Title:    "Hidden Doc",
		URL:      "https://doc.example.com",
		Category: "文件",
		Hidden:   true,
	}, text); err != nil {
		t.Fatalf("createLink(文件) 發生錯誤：%v", err)
	}
	if _, err := createLink(ctx, db, linkCard{
		Title: "Default Card",
		URL:   "https://default.example.com",
	}, text); err != nil {
		t.Fatalf("createLink(未分類) 發生錯誤：%v", err)
	}

	categories, err := listVisibleCategories(ctx, db)
	if err != nil {
		t.Fatalf("listVisibleCategories 發生錯誤：%v", err)
	}
	got := []string{categories[0].Name, categories[1].Name}
	want := []string{"工具", defaultCategoryName}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("可見卡片盒 = %v，預期 %v", got, want)
		}
	}
}

func TestUpdateCategoriesSort(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	for _, name := range []string{"工具", "文件", "服務"} {
		if err := createCategory(ctx, db, name, text); err != nil {
			t.Fatalf("createCategory(%s) 發生錯誤：%v", name, err)
		}
	}

	if err := updateCategoriesSort(ctx, db, []categorySortUpdate{
		{Name: "服務", SortOrder: 1},
		{Name: "文件", SortOrder: 2},
		{Name: "工具", SortOrder: 3},
	}, text); err != nil {
		t.Fatalf("updateCategoriesSort 發生錯誤：%v", err)
	}

	categories, err := listCategories(ctx, db)
	if err != nil {
		t.Fatalf("listCategories 發生錯誤：%v", err)
	}
	got := []string{categories[0].Name, categories[1].Name, categories[2].Name, categories[3].Name}
	want := []string{"服務", "文件", "工具", defaultCategoryName}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("排序結果 = %v，預期 %v", got, want)
		}
	}
}

func TestVisibleCategoriesFollowManagedSort(t *testing.T) {
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	ctx := context.Background()
	text := mustReadLanguage(defaultLanguageName).text
	for _, name := range []string{"Online", "MyHome", "Google"} {
		if err := createCategory(ctx, db, name, text); err != nil {
			t.Fatalf("createCategory(%s) 發生錯誤：%v", name, err)
		}
		if _, err := createLink(ctx, db, linkCard{
			Title:    name + " Card",
			URL:      "https://" + strings.ToLower(name) + ".example.com",
			Category: name,
		}, text); err != nil {
			t.Fatalf("createLink(%s) 發生錯誤：%v", name, err)
		}
	}

	if err := updateCategoriesSort(ctx, db, []categorySortUpdate{
		{Name: "MyHome", SortOrder: 1},
		{Name: "Online", SortOrder: 2},
		{Name: "Google", SortOrder: 3},
	}, text); err != nil {
		t.Fatalf("updateCategoriesSort 發生錯誤：%v", err)
	}

	categories, err := listVisibleCategories(ctx, db)
	if err != nil {
		t.Fatalf("listVisibleCategories 發生錯誤：%v", err)
	}
	got := []string{categories[0].Name, categories[1].Name, categories[2].Name}
	want := []string{"MyHome", "Online", "Google"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("前台卡片盒排序 = %v，預期 %v", got, want)
		}
	}
}
