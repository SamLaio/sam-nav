# 專案慣例

這份文件給 coding agent 與協作者快速對齊用。修改前先讀現有實作，盡量沿用目前的 Go 後端、Go template、原生 JavaScript 與 SortableJS 風格。

## 專案結構

- `site/`：會被 Docker image 包進去的 Go 站台程式、templates、static assets 與 vendor JS。
- `site/main.go`：Gin 伺服器入口。
- `site/templates/`：Go `html/template` 頁面，不使用 React 或 TypeScript。
- `site/static/`：CSS、原生 JavaScript、SortableJS 等靜態資源。
- `data/`：runtime 資料目錄，放 SQLite、icon cache、log、generated secrets；不要提交內容。
- `deffault.env`：預設環境變數，compose 與本機啟動可參考。
- `Dockerfile`、`docker-compose.template.yml`：Docker 包裝與範例部署設定。

## 常用指令

- 後端啟動：`cd site && go run .`
- 後端建置：`cd site && go build -o sam-nav .`
- 後端測試：`cd site && go test ./...`
- Docker 範例設定：複製 `docker-compose.template.yml` 為 `docker-compose.yml` 後再調整。

## Docker 慣例

- 除非使用者明確說「重建 docker」，否則不要執行任何 Docker rebuild 或等同於 rebuild 的指令。
- 需要重建 Docker 時，一律進入 WSL 執行相關指令，不要直接在 Windows PowerShell 內重建。
- 執行檢查或驗證指令時，也優先進入 WSL 執行，避免 Windows 與 WSL 的 Go cache 或路徑環境差異造成誤判。

## 後端慣例

- Go 程式碼提交前跑 `gofmt`。
- API 路由集中在 `site/main.go` 或後續拆出的 router 初始化檔，管理端路由放在 `/api/admin` 並套用 JWT 或 session middleware。
- Handler 保持薄層：驗證輸入、呼叫 `service`、回傳 JSON；資料庫細節不要散落到新 handler，優先放進 `service/` 或 `database/`。
- JSON 回應沿用現有格式：成功時使用 `success`、`message`、`data`；失敗時使用 `success: false` 與 `errorMessage`。
- 新增資料模型時，同步檢查 `types/`、`database/migration.go`、匯入匯出與管理端 API 是否需要更新。
- 不要把 runtime 產物或本機資料庫提交進 repo；SQLite 與 cache 一律放 `data/`。

## 前端慣例

- 不使用 React、TypeScript 或前端 build 流程。
- 頁面使用 Go `html/template`，互動使用原生 JavaScript。
- 拖曳排序使用 SortableJS；正式實作時將 vendored build 放在 `site/static/vendor/sortable.min.js`。
- API 呼叫使用 `fetch`，集中到相鄰的 `site/static/js/*.js`，不要在模板裡塞大量 inline script。
- 樣式放在 `site/static/css/`，新增互動狀態時確認亮色/暗色主題都可讀。

## 修改原則

- 保持改動聚焦：修 bug 或加功能時，只碰必要檔案。
- 優先修正根因，不以大型重構包裝小改動。
- 程式碼、設定檔、Docker/compose、CSS、JavaScript 與模板中的註解一律使用正體中文。
- 既有公開 API、資料表欄位、匯入匯出格式與設定項需維持相容；需要破壞性變更時，先在說明中明確標出。
- 新增需要驗證的行為時，補上最靠近該行為的測試；若專案缺少合適測試框架，至少提供手動驗證步驟。
- README、OpenAPI 或 UI 文案涉及使用方式變更時，和程式碼一起更新。

## Release 慣例

- 發布 release 前，先檢查目前版本與最新 git tag/版本之後是否還有新的功能或使用者可見變更。
- 如果有新的功能或使用者可見變更，先詢問使用者是否要調整版號，並在詢問時提示目前版號到哪一版。
- 使用目前程式碼與變更內容產生 release Markdown 檔，不要只依賴舊筆記。
- Release Markdown 檔屬於產生物，不提交進 git；相關檔名 pattern 已加入 `.gitignore`。

## 驗證清單

依改動範圍選擇最小但足夠的驗證：

- 下列驗證指令預設都在 WSL 內、專案目錄 `/mnt/d/project/sam-nav` 執行。
- Go 後端：`cd site && go test ./...`
- Go 格式：`cd site && gofmt -w .`
- API 或整合行為：啟動 `cd site && go run .` 手動檢查。
