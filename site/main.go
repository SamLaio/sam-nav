package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/* static/css/* static/js/* static/vendor/* lang/*.json deffault.env
var siteFS embed.FS

type pageData struct {
	Title         string
	Version       string
	BuildHash     string
	DefaultTheme  string
	OpenNewTab    bool
	Background    string
	Settings      appSettings
	Links         []linkCard
	Categories    []categoryBox
	SearchEngines []searchEngine
}

type loginData struct {
	Title        string
	Version      string
	BuildHash    string
	DefaultTheme string
	Background   string
	Error        string
}

const defaultLanguageName = "zhTW"

var (
	defaultEnvOnce sync.Once
	defaultEnv     map[string]string
	settingEnvOnce sync.Once
	settingEnv     map[string]string
)

func main() {
	languageProvider := newLanguageProvider(env("SAM_NAV_LANG", defaultLanguageName))
	text := languageProvider.text
	appVersion := env("SAM_NAV_VERSION", "v0.2.1")
	appBuildHash := buildHash()

	dataDir := env("SAM_NAV_DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf(text("log_create_data_dir"), err)
	}
	dbPath := env("SAM_NAV_DB_PATH", filepath.Join(dataDir, "nav.sqlite"))
	db, err := openDatabase(dbPath)
	if err != nil {
		log.Fatalf(text("log_open_database"), err)
	}
	defer db.Close()
	auth := newAuthService(db, text)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"t":         text,
		"firstRune": firstRune,
	}).ParseFS(siteFS, "templates/*.html"))
	router.SetHTMLTemplate(tmpl)
	staticFS := mustSub(siteFS, "static")
	router.StaticFS("/static", http.FS(staticFS))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "ok",
			"data": gin.H{
				"version":   appVersion,
				"buildHash": appBuildHash,
			},
		})
	})

	router.GET("/", func(c *gin.Context) {
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_settings_failed"), err)
			settings = fallbackAppSettings(text)
		}
		languageProvider.useLanguage(settings.Language)
		links, err := listVisibleLinks(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_links_failed"), err)
			links = []linkCard{}
		}
		categories, err := listVisibleCategories(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_categories_failed"), err)
			categories = []categoryBox{}
		}
		searchEngines, err := listVisibleSearchEngines(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_search_engines_failed"), err)
			searchEngines = []searchEngine{}
		}

		c.HTML(http.StatusOK, "home.html", pageData{
			Title:         settings.SiteTitle,
			Version:       appVersion,
			BuildHash:     appBuildHash,
			DefaultTheme:  settings.DefaultTheme,
			OpenNewTab:    settings.OpenNewTab,
			Background:    settings.Background,
			Settings:      settings,
			Links:         links,
			Categories:    categories,
			SearchEngines: searchEngines,
		})
	})

	renderLogin := func(c *gin.Context) {
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_settings_failed"), err)
			settings = fallbackAppSettings(text)
		}
		languageProvider.useLanguage(settings.Language)
		loginError, _ := c.Get("loginError")
		c.HTML(http.StatusOK, "login.html", loginData{
			Title:        settings.SiteTitle,
			Version:      appVersion,
			BuildHash:    appBuildHash,
			DefaultTheme: settings.DefaultTheme,
			Background:   settings.Background,
			Error:        stringValue(loginError),
		})
	}
	registerAuthRoutes(router, auth, renderLogin)

	renderAdmin := func(c *gin.Context) {
		if !auth.requirePage(c) {
			return
		}
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_settings_failed"), err)
			settings = fallbackAppSettings(text)
		}
		languageProvider.useLanguage(settings.Language)
		links, err := listAdminLinks(c.Request.Context(), db)
		if err != nil {
			log.Printf(text("log_load_links_failed"), err)
			links = []linkCard{}
		}

		c.HTML(http.StatusOK, "admin.html", pageData{
			Title:        settings.SiteTitle,
			Version:      appVersion,
			BuildHash:    appBuildHash,
			DefaultTheme: settings.DefaultTheme,
			OpenNewTab:   settings.OpenNewTab,
			Background:   settings.Background,
			Settings:     settings,
			Links:        links,
		})
	}
	router.GET("/admin", renderAdmin)
	router.GET("/admin/links", renderAdmin)
	adminAPI := router.Group("/api/admin", auth.requireAPI())
	registerLinkRoutes(router, adminAPI, db, text)
	registerCategoryRoutes(adminAPI, db, text)
	registerSearchEngineRoutes(adminAPI, db, text)
	registerSettingsRoutes(router, adminAPI, db, text, languageProvider.useLanguage)
	registerBackupRoutes(adminAPI, db, text, languageProvider.useLanguage)

	port := env("SAM_NAV_PORT", "80")
	log.Printf(text("log_starting"), port, dataDir, dbPath)
	log.Printf(text("log_build_hash"), appBuildHash)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if value, ok := settingEnvValue(key); ok {
		return value
	}
	if value, ok := defaultEnvValue(key); ok {
		return value
	}
	return fallback
}

func settingEnvValue(key string) (string, bool) {
	settingEnvOnce.Do(func() {
		settingEnv = readSettingEnv()
	})
	value, ok := settingEnv[key]
	return value, ok
}

func readSettingEnv() map[string]string {
	settingPath := settingEnvPath()
	if settingPath == "" {
		return map[string]string{}
	}
	content, err := os.ReadFile(settingPath)
	if err != nil {
		log.Printf("讀取 setting.env 失敗：%v", err)
		return map[string]string{}
	}
	return parseEnvFile(string(content))
}

func settingEnvPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	if filepath.Base(workingDir) == "site" {
		candidate := filepath.Join(filepath.Dir(workingDir), "setting.env")
		if fileExists(candidate) {
			return candidate
		}
		return ""
	}
	candidate := filepath.Join(workingDir, "setting.env")
	if fileExists(candidate) {
		return candidate
	}
	return ""
}

func defaultEnvValue(key string) (string, bool) {
	defaultEnvOnce.Do(func() {
		defaultEnv = readDefaultEnv()
	})
	value, ok := defaultEnv[key]
	return value, ok
}

func readDefaultEnv() map[string]string {
	content, err := siteFS.ReadFile("deffault.env")
	if err != nil {
		log.Printf("讀取預設環境變數失敗：%v", err)
		return map[string]string{}
	}
	return parseEnvFile(string(content))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func parseEnvFile(content string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}
	return values
}

func buildHash() string {
	configured := env("SAM_NAV_BUILD_HASH", "")
	if configured != "" {
		return configured
	}
	hash, err := randomHex(4)
	if err != nil {
		return time.Now().Format("150405")
	}
	return hash
}

func mustSub(source embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(source, dir)
	if err != nil {
		log.Fatal(err)
	}
	return sub
}

type languageMap map[string]string

type languageProvider struct {
	mu       sync.RWMutex
	name     string
	language languageMap
}

func newLanguageProvider(name string) *languageProvider {
	provider := &languageProvider{}
	provider.useLanguage(name)
	return provider
}

func (provider *languageProvider) useLanguage(name string) string {
	name = normalizeLanguageName(name)
	if name == "" {
		name = defaultLanguageName
	}
	language := loadLanguage(name)

	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.name = name
	provider.language = language
	return name
}

func (provider *languageProvider) text(key string) string {
	provider.mu.RLock()
	defer provider.mu.RUnlock()
	if provider.language == nil {
		return key
	}
	return provider.language.text(key)
}

func loadLanguage(name string) languageMap {
	name = normalizeLanguageName(name)
	language := mustReadLanguage(defaultLanguageName)
	if name == "" || name == defaultLanguageName {
		return language
	}

	requestedLanguage, err := readLanguage(name)
	if err != nil {
		log.Printf("找不到語言檔 %s，改用 %s：%v", name, defaultLanguageName, err)
		return language
	}

	for key, value := range requestedLanguage {
		language[key] = value
	}
	return language
}

func normalizeLanguageName(name string) string {
	switch name {
	case "zhTW":
		return defaultLanguageName
	case "en":
		return "en"
	default:
		return ""
	}
}

func mustReadLanguage(name string) languageMap {
	language, err := readLanguage(name)
	if err != nil {
		log.Fatalf("讀取語言檔 %s 失敗：%v", name, err)
	}
	return language
}

func readLanguage(name string) (languageMap, error) {
	languagePath := path.Join("lang", name+".json")
	content, err := siteFS.ReadFile(languagePath)
	if err != nil {
		return nil, err
	}

	var language languageMap
	if err := json.Unmarshal(content, &language); err != nil {
		return nil, err
	}
	return language, nil
}

func (language languageMap) text(key string) string {
	if value, ok := language[key]; ok {
		return value
	}
	return key
}

func firstRune(value string) string {
	for _, char := range value {
		return string(char)
	}
	return ""
}

func fallbackAppSettings(text textFunc) appSettings {
	language := normalizeLanguageName(env("SAM_NAV_LANG", defaultLanguageName))
	if language == "" {
		language = defaultLanguageName
	}
	theme := normalizeTheme(env("SAM_NAV_DEFAULT_THEME", "light"))
	if theme == "" {
		theme = "light"
	}
	return appSettings{
		SiteTitle:       env("SAM_NAV_SITE_TITLE", text("default_title")),
		Language:        language,
		DefaultTheme:    theme,
		OpenNewTab:      env("SAM_NAV_OPEN_IN_NEW_TAB", "1") == "1",
		AdminUsername:   env("SAM_NAV_AUTH_USERNAME", "admin"),
		SearchEngineURL: "",
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
