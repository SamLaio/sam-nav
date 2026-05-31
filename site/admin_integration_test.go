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
