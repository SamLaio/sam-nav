package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	settingSiteTitle         = "site_title"
	settingLanguage          = "language"
	settingDefaultTheme      = "default_theme"
	settingOpenNewTab        = "open_new_tab"
	settingBackground        = "background"
	settingSearchEngineURL   = "search_engine_url"
	settingAdminUsername     = "admin_username"
	settingAdminPasswordHash = "admin_password_hash"
	settingAdminPasswordSalt = "admin_password_salt"
)

type appSettings struct {
	SiteTitle       string `json:"siteTitle"`
	Language        string `json:"language"`
	DefaultTheme    string `json:"defaultTheme"`
	OpenNewTab      bool   `json:"openNewTab"`
	Background      string `json:"background"`
	SearchEngineURL string `json:"searchEngineURL"`
	AdminUsername   string `json:"adminUsername"`
}

type settingsPayload struct {
	SiteTitle       string `json:"siteTitle"`
	Language        string `json:"language"`
	DefaultTheme    string `json:"defaultTheme"`
	OpenNewTab      bool   `json:"openNewTab"`
	Background      string `json:"background"`
	SearchEngineURL string `json:"searchEngineURL"`
	AdminUsername   string `json:"adminUsername"`
	NewPassword     string `json:"newPassword"`
}

type themePayload struct {
	Theme string `json:"theme"`
}

func registerSettingsRoutes(router *gin.Engine, admin *gin.RouterGroup, db *sql.DB, text textFunc, applyLanguage func(string) string) {
	admin.GET("/settings", func(c *gin.Context) {
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_settings_loaded"), settings)
	})

	admin.PUT("/settings", func(c *gin.Context) {
		var payload settingsPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}

		settings, err := updateAppSettings(c.Request.Context(), db, payload, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		applyLanguage(settings.Language)
		jsonOK(c, text("api_settings_updated"), settings)
	})

	router.PUT("/api/theme", func(c *gin.Context) {
		var payload themePayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		theme := normalizeTheme(payload.Theme)
		if theme == "" {
			jsonError(c, http.StatusBadRequest, errors.New(text("error_invalid_theme")))
			return
		}
		if err := setSetting(c.Request.Context(), db, settingDefaultTheme, theme); err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		applyLanguage(settings.Language)
		jsonOK(c, text("api_theme_updated"), settings)
	})
}

func loadAppSettings(ctx context.Context, db *sql.DB) (appSettings, error) {
	title, err := getSetting(ctx, db, settingSiteTitle, env("SAM_NAV_SITE_TITLE", "Sam Nav"))
	if err != nil {
		return appSettings{}, err
	}
	language, err := getSetting(ctx, db, settingLanguage, env("SAM_NAV_LANG", defaultLanguageName))
	if err != nil {
		return appSettings{}, err
	}
	theme, err := getSetting(ctx, db, settingDefaultTheme, env("SAM_NAV_DEFAULT_THEME", "light"))
	if err != nil {
		return appSettings{}, err
	}
	openNewTab, err := getSetting(ctx, db, settingOpenNewTab, env("SAM_NAV_OPEN_IN_NEW_TAB", "1"))
	if err != nil {
		return appSettings{}, err
	}
	background, err := getSetting(ctx, db, settingBackground, "")
	if err != nil {
		return appSettings{}, err
	}
	searchEngineURL, err := getSetting(ctx, db, settingSearchEngineURL, "")
	if err != nil {
		return appSettings{}, err
	}
	adminUsername, err := getSetting(ctx, db, settingAdminUsername, env("SAM_NAV_AUTH_USERNAME", "admin"))
	if err != nil {
		return appSettings{}, err
	}

	language = normalizeLanguageName(language)
	if language == "" {
		language = defaultLanguageName
	}
	theme = normalizeTheme(theme)
	if theme == "" {
		theme = "light"
	}

	return appSettings{
		SiteTitle:       title,
		Language:        language,
		DefaultTheme:    theme,
		OpenNewTab:      openNewTab == "1",
		Background:      background,
		SearchEngineURL: searchEngineURL,
		AdminUsername:   adminUsername,
	}, nil
}

func updateAppSettings(ctx context.Context, db *sql.DB, payload settingsPayload, text textFunc) (appSettings, error) {
	payload.SiteTitle = strings.TrimSpace(payload.SiteTitle)
	payload.Language = normalizeLanguageName(payload.Language)
	payload.DefaultTheme = normalizeTheme(payload.DefaultTheme)
	payload.Background = strings.TrimSpace(payload.Background)
	payload.SearchEngineURL = strings.TrimSpace(payload.SearchEngineURL)
	payload.AdminUsername = strings.TrimSpace(payload.AdminUsername)

	if payload.SiteTitle == "" {
		return appSettings{}, errors.New(text("error_site_title_required"))
	}
	if payload.Language == "" {
		return appSettings{}, errors.New(text("error_invalid_language"))
	}
	if payload.DefaultTheme == "" {
		return appSettings{}, errors.New(text("error_invalid_theme"))
	}
	if payload.AdminUsername == "" {
		return appSettings{}, errors.New(text("error_admin_username_required"))
	}
	if payload.SearchEngineURL != "" && !strings.Contains(payload.SearchEngineURL, "%s") {
		return appSettings{}, errors.New(text("error_search_engine_placeholder_required"))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return appSettings{}, err
	}
	defer tx.Rollback()

	values := map[string]string{
		settingSiteTitle:       payload.SiteTitle,
		settingLanguage:        payload.Language,
		settingDefaultTheme:    payload.DefaultTheme,
		settingOpenNewTab:      boolSetting(payload.OpenNewTab),
		settingBackground:      payload.Background,
		settingSearchEngineURL: payload.SearchEngineURL,
		settingAdminUsername:   payload.AdminUsername,
	}
	for key, value := range values {
		if err := setSettingTx(ctx, tx, key, value); err != nil {
			return appSettings{}, err
		}
	}

	if strings.TrimSpace(payload.NewPassword) != "" {
		salt, hash, err := hashPassword(payload.NewPassword)
		if err != nil {
			return appSettings{}, err
		}
		if err := setSettingTx(ctx, tx, settingAdminPasswordSalt, salt); err != nil {
			return appSettings{}, err
		}
		if err := setSettingTx(ctx, tx, settingAdminPasswordHash, hash); err != nil {
			return appSettings{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return appSettings{}, err
	}
	return loadAppSettings(ctx, db)
}

func getSetting(ctx context.Context, db *sql.DB, key, fallback string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?;`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	return value, err
}

func setSetting(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO app_settings (key, value, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP;`,
		key,
		value,
	)
	return err
}

func setSettingTx(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO app_settings (key, value, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP;`,
		key,
		value,
	)
	return err
}

func normalizeTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return ""
	}
}

func boolSetting(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
