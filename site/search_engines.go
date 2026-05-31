package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultSearchEngineURL = "https://www.google.com/search?q=%s"

type searchEngine struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
	Enabled   bool   `json:"enabled"`
}

type searchEngineSortUpdate struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sortOrder"`
}

type searchEngineSortPayload struct {
	Items []searchEngineSortUpdate `json:"items"`
}

func registerSearchEngineRoutes(admin *gin.RouterGroup, db *sql.DB, text textFunc) {
	admin.GET("/search-engines", func(c *gin.Context) {
		engines, err := listSearchEngines(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_search_engines_loaded"), engines)
	})

	admin.POST("/search-engines", func(c *gin.Context) {
		var engine searchEngine
		if err := c.ShouldBindJSON(&engine); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		created, err := createSearchEngine(c.Request.Context(), db, engine, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_engine_created"), created)
	})

	admin.PUT("/search-engines/:id", func(c *gin.Context) {
		id, err := parseSearchEngineID(c.Param("id"), text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}

		var engine searchEngine
		if err := c.ShouldBindJSON(&engine); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		engine.ID = id
		updated, err := updateSearchEngine(c.Request.Context(), db, engine, text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_engine_updated"), updated)
	})

	admin.DELETE("/search-engines/:id", func(c *gin.Context) {
		id, err := parseSearchEngineID(c.Param("id"), text)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := deleteSearchEngine(c.Request.Context(), db, id, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_engine_deleted"), nil)
	})

	admin.PUT("/search-engines/sort", func(c *gin.Context) {
		updates, err := bindSearchEngineSortPayload(c.Request.Body)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := updateSearchEnginesSort(c.Request.Context(), db, updates, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		jsonOK(c, text("api_search_engines_sorted"), nil)
	})
}

func listSearchEngines(ctx context.Context, db *sql.DB) ([]searchEngine, error) {
	return querySearchEngines(ctx, db, "")
}

func listVisibleSearchEngines(ctx context.Context, db *sql.DB) ([]searchEngine, error) {
	return querySearchEngines(ctx, db, "WHERE enabled = 1")
}

func querySearchEngines(ctx context.Context, db *sql.DB, where string) ([]searchEngine, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, url, sort_order, enabled
		FROM nav_search_engines
		`+where+`
		ORDER BY sort_order ASC, id ASC;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	engines := make([]searchEngine, 0)
	for rows.Next() {
		var engine searchEngine
		if err := rows.Scan(&engine.ID, &engine.Name, &engine.URL, &engine.SortOrder, &engine.Enabled); err != nil {
			return nil, err
		}
		engines = append(engines, engine)
	}
	return engines, rows.Err()
}

func createSearchEngine(ctx context.Context, db *sql.DB, engine searchEngine, text textFunc) (searchEngine, error) {
	engine = cleanSearchEngine(engine)
	if err := validateSearchEngine(engine, text); err != nil {
		return searchEngine{}, err
	}
	if engine.SortOrder <= 0 {
		sortOrder, err := nextSearchEngineSort(ctx, db)
		if err != nil {
			return searchEngine{}, err
		}
		engine.SortOrder = sortOrder
	}
	result, err := db.ExecContext(
		ctx,
		`INSERT INTO nav_search_engines (name, url, sort_order, enabled, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP);`,
		engine.Name,
		engine.URL,
		engine.SortOrder,
		engine.Enabled,
	)
	if err != nil {
		return searchEngine{}, err
	}
	engine.ID, err = result.LastInsertId()
	return engine, err
}

func updateSearchEngine(ctx context.Context, db *sql.DB, engine searchEngine, text textFunc) (searchEngine, error) {
	engine = cleanSearchEngine(engine)
	if engine.ID <= 0 {
		return searchEngine{}, errors.New(text("error_invalid_search_engine_id"))
	}
	if err := validateSearchEngine(engine, text); err != nil {
		return searchEngine{}, err
	}
	if engine.SortOrder <= 0 {
		sortOrder, err := nextSearchEngineSort(ctx, db)
		if err != nil {
			return searchEngine{}, err
		}
		engine.SortOrder = sortOrder
	}
	result, err := db.ExecContext(
		ctx,
		`UPDATE nav_search_engines
		 SET name = ?, url = ?, sort_order = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?;`,
		engine.Name,
		engine.URL,
		engine.SortOrder,
		engine.Enabled,
		engine.ID,
	)
	if err != nil {
		return searchEngine{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return searchEngine{}, err
	}
	if affected == 0 {
		return searchEngine{}, errors.New(text("error_search_engine_not_found"))
	}
	return engine, nil
}

func deleteSearchEngine(ctx context.Context, db *sql.DB, id int64, text textFunc) error {
	result, err := db.ExecContext(ctx, `DELETE FROM nav_search_engines WHERE id = ?;`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New(text("error_search_engine_not_found"))
	}
	return nil
}

func bindSearchEngineSortPayload(body io.Reader) ([]searchEngineSortUpdate, error) {
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	var payload searchEngineSortPayload
	if err := json.Unmarshal(content, &payload); err == nil && payload.Items != nil {
		return payload.Items, nil
	}
	var updates []searchEngineSortUpdate
	if err := json.Unmarshal(content, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func updateSearchEnginesSort(ctx context.Context, db *sql.DB, updates []searchEngineSortUpdate, text textFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE nav_search_engines SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, update := range updates {
		if update.ID <= 0 {
			return errors.New(text("error_invalid_search_engine_id"))
		}
		if update.SortOrder <= 0 {
			return errors.New(text("error_invalid_sort_order"))
		}
		result, err := stmt.ExecContext(ctx, update.SortOrder, update.ID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return errors.New(text("error_search_engine_not_found"))
		}
	}
	return tx.Commit()
}

func importSearchEnginesBackup(ctx context.Context, db *sql.DB, engines []searchEngine, text textFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM nav_search_engines;`); err != nil {
		return err
	}
	for index, engine := range engines {
		engine = cleanSearchEngine(engine)
		if engine.SortOrder <= 0 {
			engine.SortOrder = index + 1
		}
		if err := validateSearchEngine(engine, text); err != nil {
			return err
		}
		if engine.ID > 0 {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO nav_search_engines (id, name, url, sort_order, enabled, updated_at)
				 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP);`,
				engine.ID,
				engine.Name,
				engine.URL,
				engine.SortOrder,
				engine.Enabled,
			); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO nav_search_engines (name, url, sort_order, enabled, updated_at)
			 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP);`,
			engine.Name,
			engine.URL,
			engine.SortOrder,
			engine.Enabled,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nextSearchEngineSort(ctx context.Context, db *sql.DB) (int, error) {
	var next int
	err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM nav_search_engines;`).Scan(&next)
	return next, err
}

func cleanSearchEngine(engine searchEngine) searchEngine {
	engine.Name = strings.TrimSpace(engine.Name)
	engine.URL = strings.TrimSpace(engine.URL)
	return engine
}

func validateSearchEngine(engine searchEngine, text textFunc) error {
	if engine.Name == "" {
		return errors.New(text("error_search_engine_name_required"))
	}
	if engine.URL == "" || !strings.Contains(engine.URL, "%s") {
		return errors.New(text("error_search_engine_placeholder_required"))
	}
	return nil
}

func parseSearchEngineID(raw string, text textFunc) (int64, error) {
	id, err := parseID(raw, text)
	if err != nil || id <= 0 {
		return 0, errors.New(text("error_invalid_search_engine_id"))
	}
	return id, nil
}
