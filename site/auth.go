package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	adminSessionCookie = "sam_nav_admin"
	sessionDuration    = 12 * time.Hour
	passwordRounds     = 120000
)

type authService struct {
	db                  *sql.DB
	secret              []byte
	text                textFunc
	loginNetworkLimiter networkLimiter
}

type loginPayload struct {
	Username string `form:"username"`
	Password string `form:"password"`
}

func newAuthService(db *sql.DB, text textFunc) *authService {
	secret := strings.TrimSpace(env("SAM_NAV_AUTH_SECRET_KEY", ""))
	if secret == "" {
		randomSecret, err := randomHex(32)
		if err != nil {
			logMessage := text("log_create_session_secret_failed")
			panic(fmt.Sprintf(logMessage, err))
		}
		secret = randomSecret
	}
	return &authService{
		db:                  db,
		secret:              []byte(secret),
		text:                text,
		loginNetworkLimiter: newNetworkLimiter(env("SAM_NAV_AUTH_ALLOWED_NETWORKS", "")),
	}
}

func registerAuthRoutes(router *gin.Engine, auth *authService, renderLogin gin.HandlerFunc) {
	router.GET("/admin/login", func(c *gin.Context) {
		if !auth.allowLoginRequest(c) {
			return
		}
		renderLogin(c)
	})
	router.POST("/admin/login", func(c *gin.Context) {
		if !auth.allowLoginRequest(c) {
			return
		}
		var payload loginPayload
		if err := c.ShouldBind(&payload); err != nil {
			renderLogin(c)
			return
		}
		if !auth.verifyCredentials(c.Request.Context(), payload.Username, payload.Password) {
			c.Set("loginError", auth.text("error_login_failed"))
			renderLogin(c)
			return
		}
		auth.setSession(c, strings.TrimSpace(payload.Username))
		c.Redirect(http.StatusSeeOther, "/admin")
	})
	router.POST("/admin/logout", func(c *gin.Context) {
		auth.clearSession(c)
		c.Redirect(http.StatusSeeOther, "/admin/login")
	})
}

func (auth *authService) requirePage(c *gin.Context) bool {
	if auth.authenticated(c) {
		return true
	}
	if !auth.loginNetworkLimiter.allowed(c.ClientIP()) {
		c.AbortWithStatus(http.StatusNotFound)
		return false
	}
	c.Redirect(http.StatusSeeOther, "/admin/login")
	return false
}

func (auth *authService) requireAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.authenticated(c) {
			jsonError(c, http.StatusUnauthorized, errors.New(auth.text("error_unauthorized")))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (auth *authService) authenticated(c *gin.Context) bool {
	raw, err := c.Cookie(adminSessionCookie)
	if err != nil || raw == "" {
		return false
	}
	token, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return false
	}
	parts := strings.Split(string(token), "|")
	if len(parts) != 3 {
		return false
	}
	expiresAt, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}
	signature := auth.sessionSignature(parts[0], parts[1])
	return hmac.Equal([]byte(parts[2]), []byte(signature))
}

func (auth *authService) setSession(c *gin.Context, username string) {
	expiresAt := strconv.FormatInt(time.Now().Add(sessionDuration).Unix(), 10)
	signature := auth.sessionSignature(username, expiresAt)
	token := base64.RawURLEncoding.EncodeToString([]byte(username + "|" + expiresAt + "|" + signature))
	c.SetCookie(adminSessionCookie, token, int(sessionDuration.Seconds()), "/", "", false, true)
}

func (auth *authService) clearSession(c *gin.Context) {
	c.SetCookie(adminSessionCookie, "", -1, "/", "", false, true)
}

func (auth *authService) sessionSignature(username, expiresAt string) string {
	mac := hmac.New(sha256.New, auth.secret)
	mac.Write([]byte(username + "|" + expiresAt))
	return hex.EncodeToString(mac.Sum(nil))
}

func (auth *authService) verifyCredentials(ctx context.Context, username, password string) bool {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return false
	}

	expectedUsername, err := getSetting(ctx, auth.db, settingAdminUsername, env("SAM_NAV_AUTH_USERNAME", "admin"))
	if err != nil || username != expectedUsername {
		return false
	}

	salt, err := getSetting(ctx, auth.db, settingAdminPasswordSalt, "")
	if err != nil {
		return false
	}
	hash, err := getSetting(ctx, auth.db, settingAdminPasswordHash, "")
	if err != nil {
		return false
	}
	if salt != "" && hash != "" {
		candidate := passwordHash(salt, password)
		return subtle.ConstantTimeCompare([]byte(candidate), []byte(hash)) == 1
	}

	defaultPassword := env("SAM_NAV_AUTH_PASSWORD", "admin")
	return subtle.ConstantTimeCompare([]byte(password), []byte(defaultPassword)) == 1
}

func (auth *authService) allowLoginRequest(c *gin.Context) bool {
	if auth.loginNetworkLimiter.allowed(c.ClientIP()) {
		return true
	}
	c.AbortWithStatus(http.StatusNotFound)
	return false
}

type networkLimiter struct {
	configured bool
	ranges     []ipRange
}

type ipRange struct {
	start net.IP
	end   net.IP
}

func newNetworkLimiter(raw string) networkLimiter {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return networkLimiter{}
	}
	limiter := networkLimiter{configured: true}
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		allowedRange, err := parseIPRange(token)
		if err != nil {
			log.Printf("略過無效的登入限定網域設定 %q：%v", token, err)
			continue
		}
		limiter.ranges = append(limiter.ranges, allowedRange)
	}
	return limiter
}

func parseIPRange(raw string) (ipRange, error) {
	if strings.Contains(raw, "~") {
		parts := strings.Split(raw, "~")
		if len(parts) != 2 {
			return ipRange{}, fmt.Errorf("IP 範圍格式錯誤")
		}
		start := parseComparableIP(parts[0])
		end := parseComparableIP(parts[1])
		if start == nil || end == nil {
			return ipRange{}, fmt.Errorf("IP 範圍包含無效位址")
		}
		if len(start) != len(end) {
			return ipRange{}, fmt.Errorf("IP 範圍不可混用 IPv4 與 IPv6")
		}
		if bytesCompare(start, end) > 0 {
			return ipRange{}, fmt.Errorf("IP 範圍起點不可大於終點")
		}
		return ipRange{start: start, end: end}, nil
	}

	if strings.Contains(raw, "/") {
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return ipRange{}, err
		}
		start := parseComparableIP(network.IP.String())
		if start == nil {
			return ipRange{}, fmt.Errorf("CIDR 位址無效")
		}
		end := make(net.IP, len(start))
		copy(end, start)
		maskSize, bitSize := network.Mask.Size()
		if maskSize < 0 || bitSize != len(start)*8 {
			return ipRange{}, fmt.Errorf("CIDR mask 無效")
		}
		for index := maskSize; index < bitSize; index++ {
			byteIndex := index / 8
			bitIndex := 7 - (index % 8)
			end[byteIndex] |= 1 << bitIndex
		}
		return ipRange{start: start, end: end}, nil
	}

	ip := parseComparableIP(raw)
	if ip == nil {
		return ipRange{}, fmt.Errorf("IP 位址無效")
	}
	return ipRange{start: ip, end: ip}, nil
}

func (limiter networkLimiter) allowed(rawIP string) bool {
	if !limiter.configured {
		return true
	}
	ip := parseComparableIP(rawIP)
	if ip == nil {
		return false
	}
	for _, allowedRange := range limiter.ranges {
		if len(ip) == len(allowedRange.start) &&
			bytesCompare(ip, allowedRange.start) >= 0 &&
			bytesCompare(ip, allowedRange.end) <= 0 {
			return true
		}
	}
	return false
}

func parseComparableIP(raw string) net.IP {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return nil
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4
	}
	return ip.To16()
}

func bytesCompare(left, right []byte) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func hashPassword(password string) (string, string, error) {
	salt, err := randomHex(16)
	if err != nil {
		return "", "", err
	}
	return salt, passwordHash(salt, password), nil
}

func passwordHash(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + password))
	for i := 0; i < passwordRounds; i++ {
		sum = sha256.Sum256(append(sum[:], []byte(salt+password)...))
	}
	return hex.EncodeToString(sum[:])
}
