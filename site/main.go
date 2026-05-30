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

	"github.com/gin-gonic/gin"
)

//go:embed templates/* static/css/* static/js/* static/vendor/* lang/*.json
var siteFS embed.FS

type pageData struct {
	Title        string
	DefaultTheme string
	OpenNewTab   bool
}

func main() {
	language := loadLanguage(env("SAM_NAV_LANG", "zh-Hant"))
	text := func(key string) string {
		return language.text(key)
	}

	dataDir := env("SAM_NAV_DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf(text("log_create_data_dir"), err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"t": text,
	}).ParseFS(siteFS, "templates/*.html"))
	router.SetHTMLTemplate(tmpl)
	staticFS := mustSub(siteFS, "static")
	router.StaticFS("/static", http.FS(staticFS))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "ok",
		})
	})

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.html", pageData{
			Title:        env("SAM_NAV_SITE_TITLE", text("default_title")),
			DefaultTheme: env("SAM_NAV_DEFAULT_THEME", "light"),
			OpenNewTab:   env("SAM_NAV_OPEN_IN_NEW_TAB", "1") == "1",
		})
	})

	router.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "admin.html", pageData{
			Title:        env("SAM_NAV_SITE_TITLE", text("default_title")),
			DefaultTheme: env("SAM_NAV_DEFAULT_THEME", "light"),
			OpenNewTab:   env("SAM_NAV_OPEN_IN_NEW_TAB", "1") == "1",
		})
	})

	port := env("SAM_NAV_PORT", "6412")
	dbPath := env("SAM_NAV_DB_PATH", filepath.Join(dataDir, "nav.sqlite"))
	log.Printf(text("log_starting"), port, dataDir, dbPath)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustSub(source embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(source, dir)
	if err != nil {
		log.Fatal(err)
	}
	return sub
}

type languageMap map[string]string

func loadLanguage(name string) languageMap {
	languagePath := path.Join("lang", name+".json")
	content, err := siteFS.ReadFile(languagePath)
	if err != nil {
		log.Fatalf("讀取語言檔失敗：%v", err)
	}

	var language languageMap
	if err := json.Unmarshal(content, &language); err != nil {
		log.Fatalf("解析語言檔失敗：%v", err)
	}
	return language
}

func (language languageMap) text(key string) string {
	if value, ok := language[key]; ok {
		return value
	}
	return key
}
