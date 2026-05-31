package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultCategoryName = "未分類"

type categoryBox struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	SortOrder int    `json:"sortOrder"`
	IsDefault bool   `json:"isDefault"`
}

type categoryPayload struct {
	Name string `json:"name"`
}

type categorySortUpdate struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

type categorySortPayload struct {
	Items []categorySortUpdate `json:"items"`
}

func registerCategoryRoutes(admin *gin.RouterGroup, db *sql.DB, text textFunc) {
	admin.GET("/categories", func(c *gin.Context) {
		categories, err := listCategories(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_categories_loaded"), categories)
	})

	admin.POST("/categories", func(c *gin.Context) {
		var payload categoryPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := createCategory(c.Request.Context(), db, payload.Name, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		categories, err := listCategories(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_category_created"), categories)
	})

	admin.PUT("/categories/sort", func(c *gin.Context) {
		updates, err := bindCategorySortPayload(c.Request.Body)
		if err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := updateCategoriesSort(c.Request.Context(), db, updates, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		categories, err := listCategories(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_categories_sorted"), categories)
	})

	admin.PUT("/categories/:name", func(c *gin.Context) {
		var payload categoryPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		if err := renameCategory(c.Request.Context(), db, pathCategoryName(c.Param("name")), payload.Name, text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		categories, err := listCategories(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_category_updated"), categories)
	})

	admin.DELETE("/categories/:name", func(c *gin.Context) {
		if err := deleteCategory(c.Request.Context(), db, pathCategoryName(c.Param("name")), text); err != nil {
			jsonError(c, http.StatusBadRequest, err)
			return
		}
		categories, err := listCategories(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_category_deleted"), categories)
	})
}

func listCategories(ctx context.Context, db *sql.DB) ([]categoryBox, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.name, c.sort_order, COUNT(l.id)
		FROM nav_categories c
		LEFT JOIN nav_links l ON l.category = c.name
		GROUP BY c.name, c.sort_order
		ORDER BY CASE WHEN c.name = ? THEN 1 ELSE 0 END, c.sort_order ASC, c.name COLLATE NOCASE;
	`, defaultCategoryName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]categoryBox, 0)
	for rows.Next() {
		var category categoryBox
		if err := rows.Scan(&category.Name, &category.SortOrder, &category.Count); err != nil {
			return nil, err
		}
		category.IsDefault = category.Name == defaultCategoryName
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func listVisibleCategories(ctx context.Context, db *sql.DB) ([]categoryBox, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.name, c.sort_order, COUNT(l.id)
		FROM nav_categories c
		LEFT JOIN nav_links l ON l.category = c.name AND l.hidden = 0
		GROUP BY c.name, c.sort_order
		HAVING COUNT(l.id) > 0
		ORDER BY CASE WHEN c.name = ? THEN 1 ELSE 0 END, c.sort_order ASC, c.name COLLATE NOCASE;
	`, defaultCategoryName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]categoryBox, 0)
	for rows.Next() {
		var category categoryBox
		if err := rows.Scan(&category.Name, &category.SortOrder, &category.Count); err != nil {
			return nil, err
		}
		category.IsDefault = category.Name == defaultCategoryName
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func createCategory(ctx context.Context, db *sql.DB, name string, text textFunc) error {
	name = cleanCategoryName(name)
	if name == "" {
		return errors.New(text("error_category_name_required"))
	}
	sortOrder, err := nextCategorySort(ctx, db)
	if err != nil {
		return err
	}
	result, err := db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO nav_categories (name, sort_order, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP);`,
		name,
		sortOrder,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New(text("error_category_exists"))
	}
	return nil
}

func renameCategory(ctx context.Context, db *sql.DB, oldName, newName string, text textFunc) error {
	oldName = cleanCategoryName(oldName)
	newName = cleanCategoryName(newName)
	if oldName == defaultCategoryName {
		return errors.New(text("error_default_category_protected"))
	}
	if oldName == "" || newName == "" {
		return errors.New(text("error_category_name_required"))
	}
	if newName == defaultCategoryName {
		return errors.New(text("error_category_exists"))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM nav_categories WHERE name = ?;`, newName).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return errors.New(text("error_category_exists"))
	}

	result, err := tx.ExecContext(
		ctx,
		`UPDATE nav_categories SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?;`,
		newName,
		oldName,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New(text("error_category_not_found"))
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE nav_links SET category = ?, updated_at = CURRENT_TIMESTAMP WHERE category = ?;`,
		newName,
		oldName,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteCategory(ctx context.Context, db *sql.DB, name string, text textFunc) error {
	name = cleanCategoryName(name)
	if name == defaultCategoryName {
		return errors.New(text("error_default_category_protected"))
	}
	if name == "" {
		return errors.New(text("error_category_name_required"))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `DELETE FROM nav_categories WHERE name = ?;`, name)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New(text("error_category_not_found"))
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE nav_links SET category = ?, updated_at = CURRENT_TIMESTAMP WHERE category = ?;`,
		defaultCategoryName,
		name,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func bindCategorySortPayload(body io.Reader) ([]categorySortUpdate, error) {
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var payload categorySortPayload
	if err := json.Unmarshal(content, &payload); err == nil && payload.Items != nil {
		return payload.Items, nil
	}

	var updates []categorySortUpdate
	if err := json.Unmarshal(content, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func updateCategoriesSort(ctx context.Context, db *sql.DB, updates []categorySortUpdate, text textFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE nav_categories SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ? AND name <> ?;`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, update := range updates {
		name := cleanCategoryName(update.Name)
		if name == "" || name == defaultCategoryName {
			return errors.New(text("error_default_category_protected"))
		}
		if update.SortOrder <= 0 {
			return errors.New(text("error_invalid_sort_order"))
		}
		result, err := stmt.ExecContext(ctx, update.SortOrder, name, defaultCategoryName)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return errors.New(text("error_category_not_found"))
		}
	}
	return tx.Commit()
}

func ensureCategory(ctx context.Context, db *sql.DB, name string) error {
	name = cleanCategoryName(name)
	if name == "" {
		name = defaultCategoryName
	}
	sortOrder, err := nextCategorySort(ctx, db)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO nav_categories (name, sort_order, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP);`,
		name,
		sortOrder,
	)
	return err
}

func nextCategorySort(ctx context.Context, db *sql.DB) (int, error) {
	var next int
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(sort_order), 0) + 1
		FROM nav_categories
		WHERE name <> ?;
	`, defaultCategoryName).Scan(&next)
	return next, err
}

func cleanCategoryName(name string) string {
	return strings.TrimSpace(name)
}

func pathCategoryName(name string) string {
	decoded, err := url.PathUnescape(name)
	if err != nil {
		return name
	}
	return decoded
}
