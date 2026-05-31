package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPasswordHash(t *testing.T) {
	salt, hash, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hashPassword 發生錯誤：%v", err)
	}
	if salt == "" || hash == "" {
		t.Fatal("salt 與 hash 不應為空")
	}
	if got := passwordHash(salt, "secret"); got != hash {
		t.Fatal("同一組 salt 與密碼應得到相同 hash")
	}
	if got := passwordHash(salt, "different"); got == hash {
		t.Fatal("不同密碼不應得到相同 hash")
	}
}

func TestNetworkLimiterAllowsConfiguredRange(t *testing.T) {
	limiter := newNetworkLimiter("192.168.50.0~192.168.50.255")

	if !limiter.allowed("192.168.50.10") {
		t.Fatal("範圍內 IP 應允許顯示登入頁")
	}
	if limiter.allowed("192.168.51.10") {
		t.Fatal("範圍外 IP 不應允許顯示登入頁")
	}
}

func TestNetworkLimiterAllowsCIDRAndSingleIP(t *testing.T) {
	limiter := newNetworkLimiter("10.0.0.0/24, 192.168.1.8")

	if !limiter.allowed("10.0.0.9") {
		t.Fatal("CIDR 範圍內 IP 應允許")
	}
	if !limiter.allowed("192.168.1.8") {
		t.Fatal("單一 IP 應允許")
	}
	if limiter.allowed("10.0.1.9") {
		t.Fatal("CIDR 範圍外 IP 不應允許")
	}
}

func TestNetworkLimiterAllowsAllWhenUnset(t *testing.T) {
	limiter := newNetworkLimiter("")

	if !limiter.allowed("203.0.113.10") {
		t.Fatal("未設定登入限定網域時不應限制登入頁")
	}
}

func TestLoginPageHiddenOutsideAllowedNetwork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SAM_NAV_AUTH_ALLOWED_NETWORKS", "192.168.50.0~192.168.50.255")
	t.Setenv("SAM_NAV_AUTH_SECRET_KEY", "test-secret")

	db, err := openDatabase(t.TempDir() + "/nav.sqlite")
	if err != nil {
		t.Fatalf("openDatabase 發生錯誤：%v", err)
	}
	defer db.Close()

	router := gin.New()
	auth := newAuthService(db, mustReadLanguage(defaultLanguageName).text)
	registerAuthRoutes(router, auth, func(c *gin.Context) {
		c.String(http.StatusOK, "login")
	})

	denied := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	deniedRequest.RemoteAddr = "192.168.51.10:1234"
	router.ServeHTTP(denied, deniedRequest)
	if denied.Code != http.StatusNotFound {
		t.Fatalf("範圍外 IP 不應顯示登入頁，取得 %d", denied.Code)
	}

	allowed := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	allowedRequest.RemoteAddr = "192.168.50.10:1234"
	router.ServeHTTP(allowed, allowedRequest)
	if allowed.Code != http.StatusOK || !strings.Contains(allowed.Body.String(), "login") {
		t.Fatalf("範圍內 IP 應顯示登入頁，取得 %d：%s", allowed.Code, allowed.Body.String())
	}
}
