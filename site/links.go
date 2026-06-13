package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
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

const iconValidationTimeout = 5 * time.Second
const maxIconImageBytes = 2 * 1024 * 1024
const maxIconDiscoveryHTMLBytes = 1 * 1024 * 1024

var linkIconHTTPClient = newLinkIconHTTPClient()

func newLinkIconHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// 圖示常來自內網設備或自簽憑證服務，驗證時只確認能取得圖片內容。
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{
		Transport: transport,
		Timeout:   iconValidationTimeout,
	}
}

func registerLinkRoutes(router *gin.Engine, admin *gin.RouterGroup, db *sql.DB, text textFunc, iconCacheDirs ...string) {
	iconCacheDir := firstIconCacheDir(iconCacheDirs...)
	router.GET("/api/links", func(c *gin.Context) {
		links, err := listVisibleLinks(c.Request.Context(), db)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_links_loaded"), links)
	})
	router.GET("/api/links/:id/icon", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.Status(http.StatusNotFound)
			return
		}
		content, contentType, ok := loadCachedLinkIcon(c.Request.Context(), db, iconCacheDir, id)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, contentType, content)
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
		created, err := createLink(c.Request.Context(), db, card, text, iconCacheDir)
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

		updated, err := updateLink(c.Request.Context(), db, card, text, iconCacheDir)
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
		if err := deleteLink(c.Request.Context(), db, id, text, iconCacheDir); err != nil {
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
	admin.PUT("/links/icons", func(c *gin.Context) {
		updatedCount, err := updateAllLinkIcons(c.Request.Context(), db, iconCacheDir)
		if err != nil {
			jsonError(c, http.StatusInternalServerError, err)
			return
		}
		jsonOK(c, text("api_icons_updated"), gin.H{"updated": updatedCount})
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

func getLinkIcon(ctx context.Context, db *sql.DB, id int64) (string, error) {
	var icon string
	err := db.QueryRowContext(ctx, `SELECT icon FROM nav_links WHERE id = ?;`, id).Scan(&icon)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(icon), err
}

func createLink(ctx context.Context, db *sql.DB, card linkCard, text textFunc, iconCacheDirs ...string) (linkCard, error) {
	iconCacheDir := firstIconCacheDir(iconCacheDirs...)
	card = cleanLink(card)
	if err := validateLink(card, text); err != nil {
		return linkCard{}, err
	}
	if err := ensureCategory(ctx, db, card.Category); err != nil {
		return linkCard{}, err
	}
	card = normalizeLinkIcon(ctx, card)
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
	if err != nil {
		return card, err
	}
	card = refreshStoredLinkIcon(ctx, db, iconCacheDir, card)
	return card, err
}

func updateLink(ctx context.Context, db *sql.DB, card linkCard, text textFunc, iconCacheDirs ...string) (linkCard, error) {
	iconCacheDir := firstIconCacheDir(iconCacheDirs...)
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
	card = normalizeLinkIcon(ctx, card)
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
	card = refreshStoredLinkIcon(ctx, db, iconCacheDir, card)
	return card, nil
}

func deleteLink(ctx context.Context, db *sql.DB, id int64, text textFunc, iconCacheDirs ...string) error {
	iconCacheDir := firstIconCacheDir(iconCacheDirs...)
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
	removeCachedLinkIcon(iconCacheDir, id)
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

func updateAllLinkIcons(ctx context.Context, db *sql.DB, iconCacheDirs ...string) (int, error) {
	iconCacheDir := firstIconCacheDir(iconCacheDirs...)
	links, err := listAdminLinks(ctx, db)
	if err != nil {
		return 0, err
	}

	type iconUpdate struct {
		id   int64
		icon string
	}
	updates := make([]iconUpdate, 0)
	for _, link := range links {
		normalized := normalizeLinkIcon(ctx, link)
		normalized = refreshLinkIconCache(ctx, iconCacheDir, normalized, true)
		if normalized.Icon != link.Icon {
			updates = append(updates, iconUpdate{id: link.ID, icon: normalized.Icon})
		}
	}
	if len(updates) == 0 {
		return 0, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE nav_links SET icon = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	for _, update := range updates {
		if _, err := stmt.ExecContext(ctx, update.icon, update.id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(updates), nil
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

func fillMissingLinkIcon(ctx context.Context, card linkCard) linkCard {
	if card.Icon != "" {
		return card
	}
	if icon := discoverLinkIcon(ctx, card.URL); icon != "" {
		card.Icon = icon
		return card
	}
	icon := faviconURL(card.URL)
	if icon != "" && linkIconURLHasImage(ctx, icon) {
		card.Icon = icon
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

func normalizeLinkIcon(ctx context.Context, card linkCard) linkCard {
	if card.Icon != "" && isGeneratedFavicon(card.Icon) {
		card.Icon = ""
	}
	if card.Icon != "" && !linkIconURLHasImage(ctx, card.Icon) {
		card.Icon = ""
	}
	return fillMissingLinkIcon(ctx, card)
}

func isGeneratedFavicon(icon string) bool {
	return strings.HasPrefix(strings.TrimSpace(icon), "https://www.google.com/s2/favicons?")
}

func discoverLinkIcon(ctx context.Context, rawURL string) string {
	pageURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || pageURL.Scheme == "" || pageURL.Host == "" || (pageURL.Scheme != "http" && pageURL.Scheme != "https") {
		return ""
	}
	content, ok := fetchLinkIconDiscoveryHTML(ctx, pageURL.String())
	if !ok {
		return ""
	}
	for _, candidate := range linkIconCandidatesFromHTML(content, pageURL) {
		if linkIconURLHasImage(ctx, candidate) {
			return candidate
		}
	}
	return ""
}

func fetchLinkIconDiscoveryHTML(ctx context.Context, pageURL string) ([]byte, bool) {
	requestCtx, cancel := context.WithTimeout(ctx, iconValidationTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, false
	}
	request.Header.Set("User-Agent", "SamNav/1.0")

	response, err := linkIconHTTPClient.Do(request)
	if err != nil {
		return nil, false
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, false
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxIconDiscoveryHTMLBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxIconDiscoveryHTMLBytes {
		return nil, false
	}
	contentType := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Type")))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		detectedType := strings.ToLower(http.DetectContentType(content))
		if !strings.Contains(detectedType, "text/html") {
			return nil, false
		}
	}
	return content, true
}

type linkIconCandidate struct {
	url      string
	priority int
	order    int
}

func linkIconCandidatesFromHTML(content []byte, pageURL *url.URL) []string {
	document, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil
	}
	baseURL := *pageURL
	if baseHref := firstHTMLAttr(document, "base", "href"); baseHref != "" {
		if resolved := resolveLinkIconURL(pageURL, baseHref); resolved != "" {
			if parsed, err := url.Parse(resolved); err == nil {
				baseURL = *parsed
			}
		}
	}

	candidates := make([]linkIconCandidate, 0)
	seen := map[string]bool{}
	order := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "link":
				rel := htmlAttr(node, "rel")
				href := htmlAttr(node, "href")
				if priority, ok := linkIconRelPriority(rel); ok {
					addLinkIconCandidate(&candidates, seen, &order, &baseURL, href, priority)
				}
			case "meta":
				name := strings.ToLower(firstNonEmpty(htmlAttr(node, "property"), htmlAttr(node, "name")))
				if name == "og:image" || name == "twitter:image" {
					addLinkIconCandidate(&candidates, seen, &order, &baseURL, htmlAttr(node, "content"), 40)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].priority < candidates[i].priority || (candidates[j].priority == candidates[i].priority && candidates[j].order < candidates[i].order) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	urls := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		urls = append(urls, candidate.url)
	}
	return urls
}

func addLinkIconCandidate(candidates *[]linkIconCandidate, seen map[string]bool, order *int, baseURL *url.URL, rawURL string, priority int) {
	resolved := resolveLinkIconURL(baseURL, rawURL)
	if resolved == "" || seen[resolved] {
		return
	}
	seen[resolved] = true
	*candidates = append(*candidates, linkIconCandidate{url: resolved, priority: priority, order: *order})
	*order += 1
}

func linkIconRelPriority(rel string) (int, bool) {
	tokens := strings.Fields(strings.ToLower(rel))
	hasIcon := false
	hasAppleTouchIcon := false
	hasMaskIcon := false
	for _, token := range tokens {
		switch token {
		case "icon":
			hasIcon = true
		case "apple-touch-icon", "apple-touch-icon-precomposed":
			hasAppleTouchIcon = true
		case "mask-icon":
			hasMaskIcon = true
		}
	}
	if hasIcon {
		return 10, true
	}
	if hasAppleTouchIcon {
		return 20, true
	}
	if hasMaskIcon {
		return 30, true
	}
	return 0, false
}

func firstHTMLAttr(root *html.Node, tagName, attrName string) string {
	if root == nil {
		return ""
	}
	if root.Type == html.ElementNode && strings.EqualFold(root.Data, tagName) {
		if value := htmlAttr(root, attrName); value != "" {
			return value
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if value := firstHTMLAttr(child, tagName, attrName); value != "" {
			return value
		}
	}
	return ""
}

func htmlAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func resolveLinkIconURL(baseURL *url.URL, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	resolved := baseURL.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func linkIconURLHasImage(ctx context.Context, icon string) bool {
	parsed, err := url.Parse(strings.TrimSpace(icon))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if linkIconRequestHasImage(ctx, http.MethodHead, icon) {
		return true
	}
	return linkIconRequestHasImage(ctx, http.MethodGet, icon)
}

func linkIconRequestHasImage(ctx context.Context, method, icon string) bool {
	requestCtx, cancel := context.WithTimeout(ctx, iconValidationTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, method, icon, nil)
	if err != nil {
		return false
	}
	request.Header.Set("User-Agent", "SamNav/1.0")

	response, err := linkIconHTTPClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	if method != http.MethodGet {
		return false
	}

	buffer := make([]byte, 512)
	read, _ := io.ReadFull(response.Body, buffer)
	if read <= 0 {
		return false
	}
	detectedType := strings.ToLower(http.DetectContentType(buffer[:read]))
	return strings.HasPrefix(detectedType, "image/")
}

func fetchLinkIconImage(ctx context.Context, icon string) ([]byte, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(icon))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", false
	}
	requestCtx, cancel := context.WithTimeout(ctx, iconValidationTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, icon, nil)
	if err != nil {
		return nil, "", false
	}
	request.Header.Set("User-Agent", "SamNav/1.0")

	response, err := linkIconHTTPClient.Do(request)
	if err != nil {
		return nil, "", false
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", false
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maxIconImageBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxIconImageBytes {
		return nil, "", false
	}
	contentType := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "image/") {
		return content, contentType, true
	}
	detectedType := detectIconContentType(content)
	if !strings.HasPrefix(detectedType, "image/") {
		return nil, "", false
	}
	return content, detectedType, true
}

func loadCachedLinkIcon(ctx context.Context, db *sql.DB, iconCacheDir string, id int64) ([]byte, string, bool) {
	if iconCacheDir == "" {
		return nil, "", false
	}
	content, contentType, ok := readCachedLinkIcon(iconCacheDir, id)
	if ok {
		return content, contentType, true
	}
	icon, err := getLinkIcon(ctx, db, id)
	if err != nil || icon == "" {
		return nil, "", false
	}
	card := linkCard{ID: id, Icon: icon}
	card = refreshLinkIconCache(ctx, iconCacheDir, card, false)
	if card.Icon == "" {
		return nil, "", false
	}
	return readCachedLinkIcon(iconCacheDir, id)
}

func refreshStoredLinkIcon(ctx context.Context, db *sql.DB, iconCacheDir string, card linkCard) linkCard {
	card = refreshLinkIconCache(ctx, iconCacheDir, card, true)
	if card.Icon == "" {
		_, _ = db.ExecContext(ctx, `UPDATE nav_links SET icon = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`, "", card.ID)
	}
	return card
}

func refreshLinkIconCache(ctx context.Context, iconCacheDir string, card linkCard, force bool) linkCard {
	if iconCacheDir == "" {
		return card
	}
	if card.ID <= 0 {
		return card
	}
	if strings.TrimSpace(card.Icon) == "" {
		card.Icon = ""
		removeCachedLinkIcon(iconCacheDir, card.ID)
		return card
	}
	if !force {
		if _, _, ok := readCachedLinkIcon(iconCacheDir, card.ID); ok {
			return card
		}
	}
	content, _, ok := fetchLinkIconImage(ctx, card.Icon)
	if !ok {
		card.Icon = ""
		removeCachedLinkIcon(iconCacheDir, card.ID)
		return card
	}
	if err := writeCachedLinkIcon(iconCacheDir, card.ID, content); err != nil {
		card.Icon = ""
		removeCachedLinkIcon(iconCacheDir, card.ID)
	}
	return card
}

func readCachedLinkIcon(iconCacheDir string, id int64) ([]byte, string, bool) {
	if iconCacheDir == "" || id <= 0 {
		return nil, "", false
	}
	content, err := os.ReadFile(cachedLinkIconPath(iconCacheDir, id))
	if err != nil || len(content) == 0 {
		return nil, "", false
	}
	contentType := detectIconContentType(content)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", false
	}
	return content, contentType, true
}

func writeCachedLinkIcon(iconCacheDir string, id int64, content []byte) error {
	if iconCacheDir == "" || id <= 0 {
		return errors.New("圖示快取路徑無效")
	}
	if err := os.MkdirAll(iconCacheDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(cachedLinkIconPath(iconCacheDir, id), content, 0o644)
}

func removeCachedLinkIcon(iconCacheDir string, id int64) {
	if iconCacheDir == "" || id <= 0 {
		return
	}
	_ = os.Remove(cachedLinkIconPath(iconCacheDir, id))
}

func clearLinkIconCache(iconCacheDir string) {
	if iconCacheDir == "" {
		return
	}
	entries, err := os.ReadDir(iconCacheDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "link-") {
			continue
		}
		_ = os.Remove(filepath.Join(iconCacheDir, entry.Name()))
	}
}

func cachedLinkIconPath(iconCacheDir string, id int64) string {
	return filepath.Join(iconCacheDir, "link-"+strconv.FormatInt(id, 10)+".img")
}

func detectIconContentType(content []byte) string {
	prefixLength := len(content)
	if prefixLength > 512 {
		prefixLength = 512
	}
	prefix := strings.ToLower(strings.TrimSpace(string(content[:prefixLength])))
	if strings.Contains(prefix, "<svg") {
		return "image/svg+xml"
	}
	return strings.ToLower(http.DetectContentType(content))
}

func firstIconCacheDir(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func linkIconPath(card linkCard) string {
	if card.ID <= 0 || strings.TrimSpace(card.Icon) == "" {
		return ""
	}
	return "/api/links/" + strconv.FormatInt(card.ID, 10) + "/icon"
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
