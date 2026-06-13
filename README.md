# Sam Nav

Sam Nav 是一個自架導航頁，使用 Go、Gin、Go template、SQLite 與原生 JavaScript 實作。前台提供卡片式網站導覽、卡片盒篩選與搜尋；後台提供卡片、卡片盒、搜尋引擎、系統設定與匯入匯出管理。

## 功能

- 前台卡片導覽、卡片盒分類、搜尋與自訂搜尋引擎，會記住上次選擇的卡片盒。
- 後台登入管理，支援管理員帳號與密碼設定。
- 卡片建立、編輯、排序、隱藏、圖示網址檢查與 favicon 自動帶入。
- 卡片盒建立、改名、刪除與排序，預設提供「未分類」。
- 搜尋引擎管理，預設提供 Google。
- 系統設定支援站台名稱、多語系、日夜模式、開啟新分頁與批次更新卡片圖示。
- 匯出與匯入卡片、卡片盒、搜尋引擎與系統設定，卡片圖示會套用同一套檢查流程。
- 支援 `zhTW` 正體中文與英文。
- `/healthz` 回傳版本與 build hash，方便確認部署版本。

## 專案結構

- `site/`：Go 站台程式、templates、static assets、語言檔與內建預設環境設定。
- `site/main.go`：Gin 伺服器入口。
- `site/deffault.env`：程式內建預設值。
- `setting-template.env`：本機覆蓋設定範本。
- `data/`：runtime 資料目錄，放 SQLite、圖示快取與產生的密鑰，不提交。
- `Dockerfile`：正式 image 建置。
- `docker-compose.template.yml`：部署範例。

## 設定優先序

環境設定優先序固定為：

```text
Docker compose / 系統環境變數 > setting.env > site/deffault.env
```

不使用 Docker 時，可複製範本：

```bash
cp setting-template.env setting.env
```

`setting.env` 是本機覆蓋檔，可放私密設定，已加入 `.gitignore`，不會提交。

常用設定：

```env
SAM_NAV_PORT=6412
SAM_NAV_DATA_DIR=./data
SAM_NAV_DB_PATH=./data/nav.sqlite
SAM_NAV_SITE_TITLE=Sam Nav
SAM_NAV_LANG=zhTW
SAM_NAV_DEFAULT_THEME=light
SAM_NAV_OPEN_IN_NEW_TAB=1
SAM_NAV_AUTH_USERNAME=admin
SAM_NAV_AUTH_PASSWORD=admin
SAM_NAV_AUTH_SECRET_KEY=
SAM_NAV_AUTH_ALLOWED_NETWORKS=
```

`SAM_NAV_AUTH_ALLOWED_NETWORKS` 用於限制可顯示後台登入頁的來源 IP。空白代表不限制，可使用單一 IP、CIDR 或範圍：

```env
SAM_NAV_AUTH_ALLOWED_NETWORKS=192.168.50.0~192.168.50.255
```

也可用逗號設定多組：

```env
SAM_NAV_AUTH_ALLOWED_NETWORKS=192.168.50.0/24,10.0.0.8
```

## 本機啟動

```bash
cd site
go run .
```

預設網址：

```text
http://127.0.0.1:6412
```

後台預設路徑：

```text
http://127.0.0.1:6412/admin
```

首次啟動預設帳密來自環境設定，預設為 `admin` / `admin`。服務對外開放前請先修改。

## Docker

先建立資料目錄：

```bash
mkdir -p data
```

複製 compose 範例：

```bash
cp docker-compose.template.yml docker-compose.yml
```

啟動：

```bash
docker compose up -d --build
```

預設對外網址：

```text
http://127.0.0.1:8085
```

查看狀態與 build hash：

```bash
curl http://127.0.0.1:8085/healthz
```

查看 Docker log：

```bash
docker logs -f sam-nav
```

程式 log 與 Gin access log 會輸出到 stdout/stderr，因此可直接由 Docker log 顯示。

## 匯入匯出

後台「系統設定」提供獨立匯入匯出區塊：

- 匯出 / 匯入卡片與卡片盒設定。
- 匯出 / 匯入系統設定。
- 全部匯出 / 匯入。

匯入時會顯示全畫面遮罩，完成後會顯示彈窗提示。

## 圖示快取

卡片圖示會在新增、更新、匯入或後台手動「更新圖示」時檢查。有效圖片會下載到 DB 同層的 `icon-cache/`，前台與後台都透過同源路徑顯示快取檔，避免瀏覽器因混合內容或內網憑證阻擋圖示。

若圖示網址無效，系統會先讀取卡片頁面的 HTML，依序嘗試 `rel="icon"`、Apple touch icon、mask icon 與社群圖片；若仍抓不到，再改用預設 favicon，最後才保留空白圖示。

## 驗證

Go 測試：

```bash
cd site
go test ./...
```

Docker compose 設定檢查：

```bash
docker compose -f docker-compose.template.yml config --services
```

JavaScript 語法檢查：

```bash
node --check site/static/js/home.js
node --check site/static/js/admin-links.js
node --check site/static/js/theme.js
```
