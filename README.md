# Sam Nav

Sam Nav 是一個自架導航頁，使用 Go、Gin、Go template、SQLite 與原生 JavaScript 實作。前台提供卡片式網站導覽、卡片盒篩選與搜尋；後台提供卡片、卡片盒、搜尋引擎、系統設定與匯入匯出管理。

## 功能

- 前台卡片導覽、卡片盒分類、搜尋與自訂搜尋引擎，會記住上次選擇的卡片盒；搜尋框輸入網址時會直接開啟。
- 前台背景音播放器，支援多音源疊加、音量調整與本機記憶。
- 後台登入管理，支援管理員帳號與密碼設定。
- 卡片建立、編輯、排序、隱藏、圖示網址檢查與 favicon 自動帶入。
- 卡片盒建立、改名、刪除與排序，預設提供「未分類」。
- 搜尋引擎管理，預設提供 Google。
- 搜尋紀錄僅記錄已登入管理員的搜尋詞，紀錄與建議僅供已登入者讀取；未登入者仍可正常搜尋。既有紀錄會保留，可於後台清除。
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
SAM_NAV_SEARCH_HISTORY_LIMIT=1000
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

管理員登入 Cookie 固定使用 `Secure` 與 `HttpOnly`。本機與 Docker 上述 HTTP 網址僅供前台存取；後台登入請透過 HTTPS 反向代理使用，瀏覽器不會在一般 HTTP 連線傳送登入 Cookie。可由反向代理終止 TLS，再透過內部網路連到 Go 服務。

## Docker

建置與開發環境使用 Go 1.26.11／Alpine 3.24，正式執行映像使用 Alpine 3.24。修改版本設定後需另外重建並部署才會生效。

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

介面使用的音樂、播放、音效與日夜模式圖示皆為程式內嵌 SVG / CSS 圖形，沒有引用第三方圖示檔或圖示字型。卡片圖示則來自使用者設定的圖示網址、目標網站 favicon 或 Google favicon fallback，快取於 runtime `icon-cache/`，不屬於 repo 內建素材。

## 背景音播放器

首頁背景音播放器的播放邏輯參考 [Moodist](https://github.com/remvze/moodist)：選取音源後開始播放、可疊加多個循環音源、支援全域音量與單軌音量，暫停與音量變化會使用淡入淡出。

音效素材沒有直接取用 Moodist 的音檔；本專案使用的音效來源與授權記錄於 `site/static/sounds/ambient/NOTICE.md`。

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
