package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func openDatabase(dbPath string) (*sql.DB, error) {
	dsn := dbPath
	if strings.Contains(dsn, "?") {
		dsn += "&_journal=WAL&_timeout=5000&_busy_timeout=5000&_txlock=immediate"
	} else {
		dsn += "?_journal=WAL&_timeout=5000&_busy_timeout=5000&_txlock=immediate"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateLinks(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateLinkIcons(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateCategories(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSettings(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSearchEngines(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrateLinks(db *sql.DB) error {
	sqlCreateTable := `
		CREATE TABLE IF NOT EXISTS nav_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			hidden BOOLEAN NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(sqlCreateTable); err != nil {
		return fmt.Errorf("建立卡片資料表失敗：%w", err)
	}
	return nil
}

func migrateLinkIcons(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, url, icon
		FROM nav_links
		WHERE icon LIKE 'https://logo.clearbit.com/%'
		   OR icon LIKE 'http://logo.clearbit.com/%';
	`)
	if err != nil {
		return fmt.Errorf("讀取需修正的卡片圖示失敗：%w", err)
	}
	defer rows.Close()

	type iconUpdate struct {
		id   int64
		icon string
	}
	updates := make([]iconUpdate, 0)
	for rows.Next() {
		var id int64
		var rawURL string
		var icon string
		if err := rows.Scan(&id, &rawURL, &icon); err != nil {
			return fmt.Errorf("掃描卡片圖示失敗：%w", err)
		}
		nextIcon := faviconURL(rawURL)
		if nextIcon == "" || nextIcon == icon {
			continue
		}
		updates = append(updates, iconUpdate{id: id, icon: nextIcon})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("讀取卡片圖示失敗：%w", err)
	}

	for _, update := range updates {
		if _, err := db.Exec(
			`UPDATE nav_links SET icon = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`,
			update.icon,
			update.id,
		); err != nil {
			return fmt.Errorf("修正卡片圖示失敗：%w", err)
		}
	}
	return nil
}

func migrateCategories(db *sql.DB) error {
	sqlCreateTable := `
		CREATE TABLE IF NOT EXISTS nav_categories (
			name TEXT PRIMARY KEY,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(sqlCreateTable); err != nil {
		return fmt.Errorf("建立卡片盒資料表失敗：%w", err)
	}
	if err := ensureColumn(db, "nav_categories", "sort_order", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("新增卡片盒排序欄位失敗：%w", err)
	}
	if _, err := db.Exec(
		`INSERT OR IGNORE INTO nav_categories (name, sort_order) VALUES (?, 0);`,
		defaultCategoryName,
	); err != nil {
		return fmt.Errorf("建立預設卡片盒失敗：%w", err)
	}
	if _, err := db.Exec(
		`UPDATE nav_links SET category = ? WHERE TRIM(category) = '';`,
		defaultCategoryName,
	); err != nil {
		return fmt.Errorf("套用預設卡片盒失敗：%w", err)
	}
	if _, err := db.Exec(`
		INSERT OR IGNORE INTO nav_categories (name, sort_order)
		SELECT DISTINCT TRIM(category), 0
		FROM nav_links
		WHERE TRIM(category) <> '';
	`); err != nil {
		return fmt.Errorf("匯入既有卡片盒失敗：%w", err)
	}
	if err := seedCategorySort(db); err != nil {
		return fmt.Errorf("初始化卡片盒排序失敗：%w", err)
	}
	return nil
}

func ensureColumn(db *sql.DB, tableName, columnName, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `);`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if name == columnName {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE ` + tableName + ` ADD COLUMN ` + columnName + ` ` + definition + `;`)
	return err
}

func seedCategorySort(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT name
		FROM nav_categories
		WHERE name <> ?
		ORDER BY CASE WHEN sort_order <= 0 THEN 1 ELSE 0 END, sort_order, name COLLATE NOCASE;
	`, defaultCategoryName)
	if err != nil {
		return err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for index, name := range names {
		if _, err := db.Exec(
			`UPDATE nav_categories SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?;`,
			index+1,
			name,
		); err != nil {
			return err
		}
	}
	_, err = db.Exec(
		`UPDATE nav_categories SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?;`,
		len(names)+1,
		defaultCategoryName,
	)
	return err
}

func migrateSettings(db *sql.DB) error {
	sqlCreateTable := `
		CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(sqlCreateTable); err != nil {
		return fmt.Errorf("建立系統設定資料表失敗：%w", err)
	}
	return nil
}

func migrateSearchEngines(db *sql.DB) error {
	sqlCreateTable := `
		CREATE TABLE IF NOT EXISTS nav_search_engines (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Exec(sqlCreateTable); err != nil {
		return fmt.Errorf("建立搜尋引擎資料表失敗：%w", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nav_search_engines;`).Scan(&count); err != nil {
		return fmt.Errorf("檢查搜尋引擎資料失敗：%w", err)
	}
	if count > 0 {
		return nil
	}

	var url string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?;`, settingSearchEngineURL).Scan(&url)
	if errors.Is(err, sql.ErrNoRows) || strings.TrimSpace(url) == "" {
		url = defaultSearchEngineURL
		err = nil
	}
	if err != nil {
		return fmt.Errorf("讀取舊搜尋引擎設定失敗：%w", err)
	}
	if _, err := db.Exec(
		`INSERT INTO nav_search_engines (name, url, sort_order, enabled, updated_at) VALUES (?, ?, 1, 1, CURRENT_TIMESTAMP);`,
		"Google",
		strings.TrimSpace(url),
	); err != nil {
		return fmt.Errorf("轉換舊搜尋引擎設定失敗：%w", err)
	}
	return nil
}
