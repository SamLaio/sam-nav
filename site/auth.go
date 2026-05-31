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
	db     *sql.DB
	secret []byte
	text   textFunc
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
		db:     db,
		secret: []byte(secret),
		text:   text,
	}
}

func registerAuthRoutes(router *gin.Engine, auth *authService, renderLogin gin.HandlerFunc) {
	router.GET("/admin/login", renderLogin)
	router.POST("/admin/login", func(c *gin.Context) {
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
