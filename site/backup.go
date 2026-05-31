package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const backupVersion = 1

type backupFile struct {
	Version       int            `json:"version"`
	App           string         `json:"app"`
	Scope         string         `json:"scope"`
	ExportedAt    string         `json:"exportedAt"`
	Categories    []categoryBox  `json:"categories,omitempty"`
	Links         []linkCard     `json:"links,omitempty"`
	Settings      *appSettings   `json:"settings,omitempty"`
	SearchEngines []searchEngine `json:"searchEngines,omitempty"`
}

type backupImportFile struct {
	Version        int            `json:"version"`
	App            string         `json:"app"`
	Scope          string         `json:"scope"`
	ExportedAt     string         `json:"exportedAt"`
	Categories     []categoryBox  `json:"categories"`
	Boxes          []categoryBox  `json:"boxes"`
	Links          []linkCard     `json:"links"`
	Cards          []linkCard     `json:"cards"`
	Settings       *appSettings   `json:"settings"`
	SystemSettings *appSettings   `json:"systemSettings"`
	SearchEngines  []searchEngine `json:"searchEngines"`
	Engines        []searchEngine `json:"engines"`
}

func registerBackupRoutes(admin *gin.RouterGroup, db *sql.DB, text textFunc, applyLanguage func(string) string) {
	admin.GET("/export/:scope", func(c *gin.Context) {
		scope := normalizeBackupScope(c.Param("scope"))
		if scope == "" {
			jsonError(c, http.StatusBadRequest, errors.New(text("error_invalid_backup_scope")))
			return
		}
		backup, err := exportBackup(c.Request.Context(), db, scope)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		content, err := json.MarshalIndent(backup, "", "  ")
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		filename := fmt.Sprintf("sam-nav-%s-%s.json", scope, time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
		c.Data(http.StatusOK, "application/json; charset=utf-8", content)
	})

	admin.POST("/import/:scope", func(c *gin.Context) {
		scope := normalizeBackupScope(c.Param("scope"))
		if scope == "" {
			jsonError(c, http.StatusBadRequest, errors.New(text("error_invalid_backup_scope")))
			return
		}
		backup, err := readImportBackup(c.Request.Body, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		settings, err := importBackup(c.Request.Context(), db, scope, backup, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if scope == "settings" || scope == "all" {
			applyLanguage(settings.Language)
		}
		jsonOK(c, text("api_import_completed"), settings)
	})
}

func normalizeBackupScope(scope string) string {
	switch strings.TrimSpace(scope) {
	case "cards", "settings", "all":
		return scope
	default:
		return ""
	}
}

func exportBackup(ctx context.Context, db *sql.DB, scope string) (backupFile, error) {
	backup := backupFile{
		Version:    backupVersion,
		App:        "sam-nav",
		Scope:      scope,
		ExportedAt: time.Now().Format(time.RFC3339),
	}
	if scope == "cards" || scope == "all" {
		categories, err := listCategories(ctx, db)
		if err != nil {
			return backupFile{}, err
		}
		links, err := listAdminLinks(ctx, db)
		if err != nil {
			return backupFile{}, err
		}
		backup.Categories = categories
		backup.Links = links
	}
	if scope == "settings" || scope == "all" {
		settings, err := loadAppSettings(ctx, db)
		if err != nil {
			return backupFile{}, err
		}
		backup.Settings = &settings
		searchEngines, err := listSearchEngines(ctx, db)
		if err != nil {
			return backupFile{}, err
		}
		backup.SearchEngines = searchEngines
	}
	return backup, nil
}

func readImportBackup(body io.Reader, text textFunc) (backupImportFile, error) {
	content, err := io.ReadAll(io.LimitReader(body, 10<<20))
	if err != nil {
		return backupImportFile{}, err
	}
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if len(strings.TrimSpace(string(content))) == 0 {
		return backupImportFile{}, errors.New(text("error_invalid_backup_file"))
	}

	var backup backupImportFile
	if err := json.Unmarshal(content, &backup); err == nil {
		backup.normalizeAliases()
		return backup, nil
	}

	var links []linkCard
	if err := json.Unmarshal(content, &links); err == nil {
		return backupImportFile{Links: links}, nil
	}
	return backupImportFile{}, errors.New(text("error_invalid_backup_file"))
}

func (backup *backupImportFile) normalizeAliases() {
	if len(backup.Categories) == 0 && len(backup.Boxes) > 0 {
		backup.Categories = backup.Boxes
	}
	if len(backup.Links) == 0 && len(backup.Cards) > 0 {
		backup.Links = backup.Cards
	}
	if backup.Settings == nil && backup.SystemSettings != nil {
		backup.Settings = backup.SystemSettings
	}
	if len(backup.SearchEngines) == 0 && len(backup.Engines) > 0 {
		backup.SearchEngines = backup.Engines
	}
}

func importBackup(ctx context.Context, db *sql.DB, scope string, backup backupImportFile, text textFunc) (appSettings, error) {
	if !backupScopeAllowsImport(backup.Scope, scope) {
		return appSettings{}, errors.New(text("error_invalid_backup_scope"))
	}
	if scope == "cards" || scope == "all" {
		if err := importCardsBackup(ctx, db, backup.Categories, backup.Links, text); err != nil {
			return appSettings{}, err
		}
	}
	if scope == "settings" || scope == "all" {
		if backup.Settings == nil {
			return appSettings{}, errors.New(text("error_backup_missing_settings"))
		}
		settings, err := importSettingsBackup(ctx, db, *backup.Settings, text)
		if err != nil {
			return appSettings{}, err
		}
		if backup.SearchEngines != nil {
			if err := importSearchEnginesBackup(ctx, db, backup.SearchEngines, text); err != nil {
				return appSettings{}, err
			}
		}
		return settings, nil
	}
	return loadAppSettings(ctx, db)
}

func backupScopeAllowsImport(fileScope, requestedScope string) bool {
	fileScope = normalizeBackupScope(fileScope)
	if fileScope == "" {
		return true
	}
	return fileScope == requestedScope || fileScope == "all"
}

func importCardsBackup(ctx context.Context, db *sql.DB, categories []categoryBox, links []linkCard, text textFunc) error {
	preparedCategories, preparedLinks, err := prepareCardsBackup(categories, links, text)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM nav_links;`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM nav_categories;`); err != nil {
		return err
	}

	for _, category := range preparedCategories {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO nav_categories (name, sort_order, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP);`,
			category.Name,
			category.SortOrder,
		); err != nil {
			return err
		}
	}

	for _, link := range preparedLinks {
		if link.ID > 0 {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO nav_links (id, title, url, description, category, icon, sort_order, hidden, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`,
				link.ID,
				link.Title,
				link.URL,
				link.Description,
				link.Category,
				link.Icon,
				link.SortOrder,
				link.Hidden,
			); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO nav_links (title, url, description, category, icon, sort_order, hidden, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`,
			link.Title,
			link.URL,
			link.Description,
			link.Category,
			link.Icon,
			link.SortOrder,
			link.Hidden,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func prepareCardsBackup(categories []categoryBox, links []linkCard, text textFunc) ([]categoryBox, []linkCard, error) {
	categoryOrders := map[string]int{}
	nextCategoryOrder := 1
	for _, category := range categories {
		name := cleanCategoryName(category.Name)
		if name == "" {
			continue
		}
		if name == defaultCategoryName {
			continue
		}
		sortOrder := category.SortOrder
		if sortOrder <= 0 {
			sortOrder = nextCategoryOrder
		}
		categoryOrders[name] = sortOrder
		nextCategoryOrder += 1
	}

	preparedLinks := make([]linkCard, 0, len(links))
	seenIDs := map[int64]bool{}
	for index, link := range links {
		link = cleanLink(link)
		if err := validateLink(link, text); err != nil {
			return nil, nil, err
		}
		if link.Icon == "" {
			link.Icon = faviconURL(link.URL)
		}
		if link.SortOrder <= 0 {
			link.SortOrder = index + 1
		}
		if link.ID > 0 {
			if seenIDs[link.ID] {
				link.ID = 0
			} else {
				seenIDs[link.ID] = true
			}
		}
		if link.Category != defaultCategoryName {
			if _, ok := categoryOrders[link.Category]; !ok {
				categoryOrders[link.Category] = nextCategoryOrder
				nextCategoryOrder += 1
			}
		}
		preparedLinks = append(preparedLinks, link)
	}

	preparedCategories := make([]categoryBox, 0, len(categoryOrders)+1)
	for name, sortOrder := range categoryOrders {
		preparedCategories = append(preparedCategories, categoryBox{
			Name:      name,
			SortOrder: sortOrder,
		})
	}
	sort.SliceStable(preparedCategories, func(i, j int) bool {
		if preparedCategories[i].SortOrder != preparedCategories[j].SortOrder {
			return preparedCategories[i].SortOrder < preparedCategories[j].SortOrder
		}
		return preparedCategories[i].Name < preparedCategories[j].Name
	})
	preparedCategories = append(preparedCategories, categoryBox{
		Name:      defaultCategoryName,
		SortOrder: len(preparedCategories) + 1,
		IsDefault: true,
	})
	return preparedCategories, preparedLinks, nil
}

func importSettingsBackup(ctx context.Context, db *sql.DB, settings appSettings, text textFunc) (appSettings, error) {
	if strings.TrimSpace(settings.SiteTitle) == "" {
		settings.SiteTitle = env("SAM_NAV_SITE_TITLE", "Sam Nav")
	}
	if normalizeLanguageName(settings.Language) == "" {
		settings.Language = defaultLanguageName
	}
	if normalizeTheme(settings.DefaultTheme) == "" {
		settings.DefaultTheme = "light"
	}
	if strings.TrimSpace(settings.AdminUsername) == "" {
		settings.AdminUsername = env("SAM_NAV_AUTH_USERNAME", "admin")
	}
	return updateAppSettings(ctx, db, settingsPayload{
		SiteTitle:       settings.SiteTitle,
		Language:        settings.Language,
		DefaultTheme:    settings.DefaultTheme,
		OpenNewTab:      settings.OpenNewTab,
		Background:      settings.Background,
		SearchEngineURL: settings.SearchEngineURL,
		AdminUsername:   settings.AdminUsername,
	}, text)
}
