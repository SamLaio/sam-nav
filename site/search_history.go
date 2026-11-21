package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultSearchHistoryLimit           = 1000
	maxSearchHistoryLimit               = 1000
	defaultSearchHistoryDisplayLimit    = 50
	defaultSearchHistorySuggestionLimit = 10
)

type searchHistoryItem struct {
	ID         int64  `json:"id"`
	Keyword    string `json:"keyword"`
	HitCount   int    `json:"hitCount"`
	LastUsedAt string `json:"lastUsedAt"`
}

type searchHistoryPayload struct {
	Keyword string `json:"keyword"`
}

type deleteSearchHistoryPayload struct {
	IDs []int64 `json:"ids"`
	All bool    `json:"all"`
}

func registerSearchHistoryRoutes(router *gin.Engine, admin *gin.RouterGroup, db *sql.DB, text textFunc, auth *authService) {
	router.GET("/api/search-history", auth.requireAPI(), func(c *gin.Context) {
		items, err := listSearchHistory(c.Request.Context(), db, c.Query("q"), queryLimit(c, defaultSearchHistorySuggestionLimit, defaultSearchHistorySuggestionLimit))
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_search_history_loaded"), items)
	})

	router.POST("/api/search-history", auth.requireAPI(), func(c *gin.Context) {
		var payload searchHistoryPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		if err := recordSearchHistory(c.Request.Context(), db, payload.Keyword, settings.SearchHistoryLimit, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_history_recorded"), nil)
	})

	admin.GET("/search-history", func(c *gin.Context) {
		settings, err := loadAppSettings(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		limit := queryLimit(c, defaultSearchHistoryDisplayLimit, settings.SearchHistoryLimit)
		items, err := listSearchHistory(c.Request.Context(), db, "", limit)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_search_history_loaded"), items)
	})

	admin.DELETE("/search-history/:id", func(c *gin.Context) {
		id, err := parseSearchHistoryID(c.Param("id"), text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := deleteSearchHistory(c.Request.Context(), db, []int64{id}, false, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_history_deleted"), nil)
	})

	admin.DELETE("/search-history", func(c *gin.Context) {
		var payload deleteSearchHistoryPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := deleteSearchHistory(c.Request.Context(), db, payload.IDs, payload.All, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_history_deleted"), nil)
	})
}

func listSearchHistory(ctx context.Context, db *sql.DB, keyword string, limit int) ([]searchHistoryItem, error) {
	keyword = normalizeSearchKeyword(keyword)
	if limit <= 0 {
		limit = defaultSearchHistorySuggestionLimit
	}

	query := `
		SELECT id, keyword, hit_count, last_used_at
		FROM nav_search_history
		ORDER BY last_used_at DESC, id DESC
		LIMIT ?;
	`
	args := []any{limit}
	if keyword != "" {
		query = `
			SELECT id, keyword, hit_count, last_used_at
			FROM nav_search_history
			WHERE normalized_keyword LIKE '%' || ? || '%'
			ORDER BY last_used_at DESC, id DESC
			LIMIT ?;
		`
		args = []any{strings.ToLower(keyword), limit}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]searchHistoryItem, 0)
	for rows.Next() {
		var item searchHistoryItem
		if err := rows.Scan(&item.ID, &item.Keyword, &item.HitCount, &item.LastUsedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func recordSearchHistory(ctx context.Context, db *sql.DB, keyword string, limit int, text textFunc) error {
	keyword = normalizeSearchKeyword(keyword)
	if keyword == "" {
		return errors.New(text("error_search_history_keyword_required"))
	}
	normalized := strings.ToLower(keyword)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO nav_search_history (keyword, normalized_keyword, hit_count, last_used_at, created_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(normalized_keyword) DO UPDATE SET
			keyword = excluded.keyword,
			hit_count = hit_count + 1,
			last_used_at = CURRENT_TIMESTAMP;
	`, keyword, normalized); err != nil {
		return err
	}
	return pruneSearchHistory(ctx, db, limit)
}

func deleteSearchHistory(ctx context.Context, db *sql.DB, ids []int64, all bool, text textFunc) error {
	if all {
		_, err := db.ExecContext(ctx, `DELETE FROM nav_search_history;`)
		return err
	}
	if len(ids) == 0 {
		return errors.New(text("error_search_history_selection_required"))
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `DELETE FROM nav_search_history WHERE id = ?;`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if id <= 0 {
			return errors.New(text("error_invalid_search_history_id"))
		}
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func pruneSearchHistory(ctx context.Context, db *sql.DB, limit int) error {
	_, err := db.ExecContext(ctx, `
		DELETE FROM nav_search_history
		WHERE id NOT IN (
			SELECT id
			FROM nav_search_history
			ORDER BY last_used_at DESC, id DESC
			LIMIT ?
		);
	`, normalizeSearchHistoryLimit(limit))
	return err
}

func pruneSearchHistoryTx(ctx context.Context, tx *sql.Tx, limit int) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM nav_search_history
		WHERE id NOT IN (
			SELECT id
			FROM nav_search_history
			ORDER BY last_used_at DESC, id DESC
			LIMIT ?
		);
	`, normalizeSearchHistoryLimit(limit))
	return err
}

func normalizeSearchKeyword(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if len([]rune(keyword)) <= 200 {
		return keyword
	}
	return string([]rune(keyword)[:200])
}

func normalizeSearchHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchHistoryLimit
	}
	if limit > maxSearchHistoryLimit {
		return maxSearchHistoryLimit
	}
	return limit
}

func queryLimit(c *gin.Context, fallback, max int) int {
	limit, err := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	if err != nil || limit <= 0 {
		limit = fallback
	}
	if max > 0 && limit > max {
		return max
	}
	return limit
}

func parseSearchHistoryID(raw string, text textFunc) (int64, error) {
	id, err := parseID(raw, text)
	if err != nil || id <= 0 {
		return 0, errors.New(text("error_invalid_search_history_id"))
	}
	return id, nil
}
