package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type linkCard struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sortOrder"`
	Hidden      bool   `json:"hidden"`
}

func (card *linkCard) UnmarshalJSON(content []byte) error {
	var payload struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
		Desc        string `json:"desc"`
		Category    string `json:"category"`
		Catelog     string `json:"catelog"`
		Catalog     string `json:"catalog"`
		Icon        string `json:"icon"`
		Logo        string `json:"logo"`
		SortOrder   int    `json:"sortOrder"`
		Sort        int    `json:"sort"`
		Hidden      *bool  `json:"hidden"`
		Hide        *bool  `json:"hide"`
		IsHidden    *bool  `json:"isHidden"`
		ChineseHide *bool  `json:"是否隱藏"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return err
	}

	card.ID = payload.ID
	card.Title = firstNonEmpty(payload.Title, payload.Name)
	card.URL = payload.URL
	card.Description = firstNonEmpty(payload.Description, payload.Desc)
	card.Category = firstNonEmpty(payload.Category, payload.Catelog, payload.Catalog)
	card.Icon = firstNonEmpty(payload.Icon, payload.Logo)
	card.SortOrder = firstPositive(payload.SortOrder, payload.Sort)
	card.Hidden = firstBool(false, payload.Hidden, payload.Hide, payload.IsHidden, payload.ChineseHide)
	return nil
}

type linkSortUpdate struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sortOrder"`
}

type linkSortPayload struct {
	Items []linkSortUpdate `json:"items"`
}

type textFunc func(string) string

func registerLinkRoutes(router *gin.Engine, admin *gin.RouterGroup, db *sql.DB, text textFunc) {
	router.GET("/api/links", func(c *gin.Context) {
		links, err := listVisibleLinks(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_links_loaded"), links)
	})

	admin.GET("/links", func(c *gin.Context) {
		links, err := listAdminLinks(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_links_loaded"), links)
	})
	admin.POST("/links", func(c *gin.Context) {
		var card linkCard
		if err := c.ShouldBindJSON(&card); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		created, err := createLink(c.Request.Context(), db, card, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_link_created"), created)
	})
	admin.PUT("/links/:id", func(c *gin.Context) {
		id, err := parseID(c.Param("id"), text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}

		var card linkCard
		if err := c.ShouldBindJSON(&card); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		card.ID = id

		updated, err := updateLink(c.Request.Context(), db, card, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_link_updated"), updated)
	})
	admin.DELETE("/links/:id", func(c *gin.Context) {
		id, err := parseID(c.Param("id"), text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := deleteLink(c.Request.Context(), db, id, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_link_deleted"), nil)
	})
	admin.PUT("/links/sort", func(c *gin.Context) {
		updates, err := bindLinkSortPayload(c.Request.Body)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := updateLinksSort(c.Request.Context(), db, updates, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_sort_updated"), nil)
	})
}

func bindLinkSortPayload(body io.Reader) ([]linkSortUpdate, error) {
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var payload linkSortPayload
	if err := json.Unmarshal(content, &payload); err == nil && payload.Items != nil {
		return payload.Items, nil
	}

	var updates []linkSortUpdate
	if err := json.Unmarshal(content, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func listVisibleLinks(ctx context.Context, db *sql.DB) ([]linkCard, error) {
	return queryLinks(ctx, db, "WHERE hidden = 0")
}

func listAdminLinks(ctx context.Context, db *sql.DB) ([]linkCard, error) {
	return queryLinks(ctx, db, "")
}

func queryLinks(ctx context.Context, db *sql.DB, where string) ([]linkCard, error) {
	query := `
		SELECT id, title, url, description, category, icon, sort_order, hidden
		FROM nav_links
		` + where + `
		ORDER BY sort_order ASC, id ASC;
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]linkCard, 0)
	for rows.Next() {
		var card linkCard
		if err := rows.Scan(
			&card.ID,
			&card.Title,
			&card.URL,
			&card.Description,
			&card.Category,
			&card.Icon,
			&card.SortOrder,
			&card.Hidden,
		); err != nil {
			return nil, err
		}
		links = append(links, card)
	}
	return links, rows.Err()
}

func createLink(ctx context.Context, db *sql.DB, card linkCard, text textFunc) (linkCard, error) {
	card = cleanLink(card)
	if err := validateLink(card, text); err != nil {
		return linkCard{}, err
	}
	if err := ensureCategory(ctx, db, card.Category); err != nil {
		return linkCard{}, err
	}
	if card.Icon == "" {
		card.Icon = faviconURL(card.URL)
	}
	if card.SortOrder <= 0 {
		nextSort, err := nextLinkSort(ctx, db)
		if err != nil {
			return linkCard{}, err
		}
		card.SortOrder = nextSort
	}

	result, err := db.ExecContext(
		ctx,
		`INSERT INTO nav_links (title, url, description, category, icon, sort_order, hidden, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`,
		card.Title,
		card.URL,
		card.Description,
		card.Category,
		card.Icon,
		card.SortOrder,
		card.Hidden,
	)
	if err != nil {
		return linkCard{}, err
	}
	card.ID, err = result.LastInsertId()
	return card, err
}

func updateLink(ctx context.Context, db *sql.DB, card linkCard, text textFunc) (linkCard, error) {
	card = cleanLink(card)
	if card.ID <= 0 {
		return linkCard{}, errors.New(text("error_invalid_link_id"))
	}
	if err := validateLink(card, text); err != nil {
		return linkCard{}, err
	}
	if err := ensureCategory(ctx, db, card.Category); err != nil {
		return linkCard{}, err
	}
	if card.SortOrder <= 0 {
		nextSort, err := nextLinkSort(ctx, db)
		if err != nil {
			return linkCard{}, err
		}
		card.SortOrder = nextSort
	}

	result, err := db.ExecContext(
		ctx,
		`UPDATE nav_links
		 SET title = ?, url = ?, description = ?, category = ?, icon = ?, sort_order = ?, hidden = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?;`,
		card.Title,
		card.URL,
		card.Description,
		card.Category,
		card.Icon,
		card.SortOrder,
		card.Hidden,
		card.ID,
	)
	if err != nil {
		return linkCard{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return linkCard{}, err
	}
	if affected == 0 {
		return linkCard{}, errors.New(text("error_link_update_not_found"))
	}
	return card, nil
}

func deleteLink(ctx context.Context, db *sql.DB, id int64, text textFunc) error {
	result, err := db.ExecContext(ctx, `DELETE FROM nav_links WHERE id = ?;`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New(text("error_link_delete_not_found"))
	}
	return nil
}

func updateLinksSort(ctx context.Context, db *sql.DB, updates []linkSortUpdate, text textFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE nav_links SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, update := range updates {
		if update.ID <= 0 {
			return errors.New(text("error_invalid_link_id"))
		}
		if update.SortOrder <= 0 {
			return errors.New(text("error_invalid_sort_order"))
		}
		if _, err := stmt.ExecContext(ctx, update.SortOrder, update.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nextLinkSort(ctx context.Context, db *sql.DB) (int, error) {
	var next int
	err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM nav_links;`).Scan(&next)
	return next, err
}

func cleanLink(card linkCard) linkCard {
	card.Title = strings.TrimSpace(card.Title)
	card.URL = strings.TrimSpace(card.URL)
	card.Description = strings.TrimSpace(card.Description)
	card.Category = cleanCategoryName(card.Category)
	if card.Category == "" {
		card.Category = defaultCategoryName
	}
	card.Icon = strings.TrimSpace(card.Icon)
	if shouldReplaceIcon(card.Icon) {
		card.Icon = faviconURL(card.URL)
	}
	return card
}

func shouldReplaceIcon(icon string) bool {
	icon = strings.ToLower(strings.TrimSpace(icon))
	return strings.HasPrefix(icon, "https://logo.clearbit.com/") || strings.HasPrefix(icon, "http://logo.clearbit.com/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstBool(fallback bool, values ...*bool) bool {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return fallback
}

func validateLink(card linkCard, text textFunc) error {
	if card.Title == "" {
		return errors.New(text("error_title_required"))
	}
	if card.URL == "" {
		return errors.New(text("error_url_required"))
	}
	return nil
}

func parseID(raw string, text textFunc) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New(text("error_invalid_link_id"))
	}
	return id, nil
}

func faviconURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return "https://www.google.com/s2/favicons?domain=" + url.QueryEscape(parsed.Hostname()) + "&sz=64"
}

func jsonOK(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

func jsonError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{
		"success":      false,
		"errorMessage": err.Error(),
	})
}
