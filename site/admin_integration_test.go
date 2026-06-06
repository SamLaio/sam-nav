package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withTestGoogleFaviconClient(t)
	t.Setenv("SAM_NAV_AUTH_USERNAME", "admin")
	t.Setenv("SAM_NAV_AUTH_PASSWORD", "admin")
	t.Setenv("SAM_NAV_AUTH_SECRET_KEY", "test-secret")

	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	text := func(key string) string {
		return mustReadLanguage(defaultLanguageName).text(key)
	}
	router := gin.New()
	auth := newAuthService(db, text)
	admin := router.Group("/api/admin", auth.requireAPI())
	registerLinkRoutes(router, admin, db, text)
	registerSettingsRoutes(router, admin, db, text, func(name string) string {
		return normalizeLanguageName(name)
	})
	registerBackupRoutes(admin, db, text, func(name string) string {
		return normalizeLanguageName(name)
	})
	registerAuthRoutes(router, auth, func(c *gin.Context) {
		c.String(http.StatusOK, "login")
	})

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/admin/links", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("未登入管理 API 應回 401，取得 %d", unauthorized.Code)
	}

	login := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader("username=admin&password=admin"))
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(login, loginRequest)
	if login.Code != http.StatusSeeOther {
		t.Fatalf("登入成功應 redirect，取得 %d", login.Code)
	}
	cookies := login.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("登入成功應設定 session cookie")
	}

	create := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/links", strings.NewReader(`{"title":"Example","url":"https://example.com/docs"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		createRequest.AddCookie(cookie)
	}
	router.ServeHTTP(create, createRequest)
	if create.Code != http.StatusOK {
		t.Fatalf("建立卡片應成功，取得 %d：%s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), "https://www.google.com/s2/favicons?domain=example.com") {
		t.Fatalf("建立卡片未自動補 favicon：%s", create.Body.String())
	}

	update := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/links/1", strings.NewReader(`{"title":"Example Updated","url":"https://docs.example.org/start","icon":""}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		updateRequest.AddCookie(cookie)
	}
	router.ServeHTTP(update, updateRequest)
	if update.Code != http.StatusOK {
		t.Fatalf("更新卡片應成功，取得 %d：%s", update.Code, update.Body.String())
	}
	if !strings.Contains(update.Body.String(), "https://www.google.com/s2/favicons?domain=docs.example.org") {
		t.Fatalf("更新卡片圖示留空時應自動補 favicon：%s", update.Body.String())
	}

	iconServer := newTestIconServer(t)
	if _, err := db.Exec(
		`INSERT INTO nav_links (title, url, description, category, icon, sort_order, hidden, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`,
		"Refresh Icon",
		"https://refresh.example.com",
		"",
		defaultCategoryName,
		iconServer.URL+"/page.html",
		2,
		false,
	); err != nil {
		t.Fatalf("建立待更新圖示測試資料失敗：%v", err)
	}
	refreshIcons := httptest.NewRecorder()
	refreshIconsRequest := httptest.NewRequest(http.MethodPut, "/api/admin/links/icons", nil)
	for _, cookie := range cookies {
		refreshIconsRequest.AddCookie(cookie)
	}
	router.ServeHTTP(refreshIcons, refreshIconsRequest)
	if refreshIcons.Code != http.StatusOK {
		t.Fatalf("更新所有圖示應成功，取得 %d：%s", refreshIcons.Code, refreshIcons.Body.String())
	}
	if !strings.Contains(refreshIcons.Body.String(), `"updated":1`) {
		t.Fatalf("更新所有圖示應回傳更新 1 筆：%s", refreshIcons.Body.String())
	}

	exportCards := httptest.NewRecorder()
	exportRequest := httptest.NewRequest(http.MethodGet, "/api/admin/export/cards", nil)
	for _, cookie := range cookies {
		exportRequest.AddCookie(cookie)
	}
	router.ServeHTTP(exportCards, exportRequest)
	if exportCards.Code != http.StatusOK {
		t.Fatalf("匯出卡片應成功，取得 %d：%s", exportCards.Code, exportCards.Body.String())
	}
	if !strings.Contains(exportCards.Body.String(), `"links"`) {
		t.Fatalf("匯出卡片應包含 links：%s", exportCards.Body.String())
	}

	theme := httptest.NewRecorder()
	themeRequest := httptest.NewRequest(http.MethodPut, "/api/theme", strings.NewReader(`{"theme":"dark"}`))
	themeRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(theme, themeRequest)
	if theme.Code != http.StatusOK {
		t.Fatalf("更新共用主題應成功，取得 %d：%s", theme.Code, theme.Body.String())
	}
	if !strings.Contains(theme.Body.String(), `"defaultTheme":"dark"`) {
		t.Fatalf("共用主題未更新為 dark：%s", theme.Body.String())
	}
}
