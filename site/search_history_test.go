package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSearchHistoryRoutesRequireLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SAM_NAV_AUTH_SECRET_KEY", "test-secret")
	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	text := mustReadLanguage(defaultLanguageName).text
	auth := newAuthService(db, text)
	router := gin.New()
	registerSearchHistoryRoutes(router, router.Group("/api/admin", auth.requireAPI()), db, text, auth)

	session := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(session)
	auth.setSession(c, "admin")
	cookie := session.Result().Cookies()[0]
	for _, loggedIn := range []bool{false, true} {
		for _, method := range []string{http.MethodPost, http.MethodGet} {
			request := httptest.NewRequest(method, "/api/search-history?q=private", strings.NewReader(`{"keyword":"private"}`))
			request.Header.Set("Content-Type", "application/json")
			if loggedIn {
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			wantStatus := http.StatusUnauthorized
			if loggedIn {
				wantStatus = http.StatusOK
			}
			if response.Code != wantStatus {
				t.Fatalf("登入=%t，%s 狀態碼=%d，預期=%d：%s", loggedIn, method, response.Code, wantStatus, response.Body.String())
			}
			if !loggedIn && strings.Contains(response.Body.String(), "private") {
				t.Fatal("未登入回應不應包含搜尋詞")
			}
		}
		items, err := listSearchHistory(context.Background(), db, "", 10)
		if err != nil {
			t.Fatal(err)
		}
		if !loggedIn && len(items) != 0 {
			t.Fatal("未登入搜尋不應寫入資料庫")
		}
		if loggedIn && (len(items) != 1 || items[0].Keyword != "private" || items[0].HitCount != 1) {
			t.Fatalf("已登入搜尋應記錄一次：%+v", items)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/search-history?q=private", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "private") {
		t.Fatal("有既存紀錄時仍不可讓未登入者讀取搜尋詞")
	}
}

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
