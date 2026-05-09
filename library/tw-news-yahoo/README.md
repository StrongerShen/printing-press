# tw-news-yahoo

Yahoo 台灣新聞 CLI — 追蹤 2026 縣市長選情

從 `tw.news.yahoo.com/topic/2026election/` 抓取選舉新聞，依關鍵字（如「彰化縣長」）過濾後存入本機 SQLite，支援離線搜尋。

## 安裝

### macOS / Linux
```bash
cd ~/printing-press/library/tw-news-yahoo
go build -o tw-news-yahoo .
mv tw-news-yahoo /usr/local/bin/   # 或加入 PATH
```

### Windows（cross-compile，在 macOS 上產出）
```bash
cd ~/printing-press/library/tw-news-yahoo
GOOS=windows GOARCH=amd64 go build -o tw-news-yahoo.exe .
```

將 `tw-news-yahoo.exe` 複製到 Windows 機器後直接執行，無需安裝 Go。

```powershell
.\tw-news-yahoo.exe fetch --county changhua
.\tw-news-yahoo.exe search 彰化縣長
```

> 注意：`watch` 指令的桌面通知在 Windows 上不會觸發，其餘功能完全正常。

## 使用方式

### 每日抓取（預設過濾「彰化」）
```bash
tw-news-yahoo fetch
```

### 依縣市代碼抓取（展開為預設候選人關鍵字包）
```bash
tw-news-yahoo fetch --county changhua   # 彰化：彰化縣長、王惠美、陳世凱…
tw-news-yahoo fetch --county taipei     # 台北：台北市長、蔣萬安…
tw-news-yahoo fetch --county taichung   # 台中：台中市長、盧秀燕…
tw-news-yahoo fetch --county kaohsiung  # 高雄：高雄市長、陳其邁…
tw-news-yahoo fetch --county tainan     # 台南：台南市長、黃偉哲…
tw-news-yahoo fetch --county taoyuan    # 桃園：桃園市長…
```

### 自訂關鍵字
```bash
tw-news-yahoo fetch --keyword 彰化縣長,王惠美,陳世凱
```

### 混用縣市 + 額外關鍵字
```bash
tw-news-yahoo fetch --county changhua --keyword 陳世凱
```

### 抓取全部選舉新聞（不過濾）
```bash
tw-news-yahoo fetch --all
```

### 增量同步（只抓新文章）
```bash
tw-news-yahoo sync
tw-news-yahoo sync --keyword 彰化
tw-news-yahoo sync --since 2026-05-01
```

### 離線搜尋（FTS5）
```bash
tw-news-yahoo search 彰化縣長
tw-news-yahoo search 王惠美 --limit 10
```

### 列出文章
```bash
tw-news-yahoo list
tw-news-yahoo list --keyword 彰化 --from 2026-05-01 --to 2026-05-31
tw-news-yahoo list --limit 20
```

### 縣市候選人快速查詢
```bash
tw-news-yahoo candidates --county changhua
tw-news-yahoo candidates --county taipei
```

支援縣市：`changhua`（彰化）、`taipei`（台北）、`taichung`（台中）、`kaohsiung`（高雄）、`tainan`（台南）、`taoyuan`（桃園）

### 趨勢圖
```bash
tw-news-yahoo timeline
tw-news-yahoo timeline --from 2026-01-01 --to 2026-05-31
```

### 持續監控（每小時檢查）
```bash
tw-news-yahoo watch
tw-news-yahoo watch --interval 30m --keyword 彰化縣長
```

### 匯出
```bash
tw-news-yahoo export --format json --output news.json
tw-news-yahoo export --format csv  --output news.csv
tw-news-yahoo export --format markdown --output news.md
tw-news-yahoo export --keyword 彰化 --format markdown
```

### JSON 輸出（搭配 jq）
```bash
tw-news-yahoo --json list | jq '.[].headline'
tw-news-yahoo --json search 彰化縣長 | jq '.[] | {headline, date_published, url}'
```

### Web UI（本機瀏覽器操作介面）
```bash
tw-news-yahoo ui
```

啟動後自動開啟瀏覽器，預設位址 `http://localhost:8080`。提供四個頁籤：
- **抓取 / 同步** — 縣市下拉、關鍵字輸入、同步起始日期（留空 = 上次同步時間）、即時串流進度
- **搜尋** — FTS5 全文搜尋
- **文章列表** — 關鍵字 / 日期過濾，一鍵匯出 CSV / Markdown
- **趨勢圖** — 依日期範圍顯示每日文章數長條圖

```bash
# 指定埠號
tw-news-yahoo ui --port 9000
```

### MCP Server（AI Agent 整合）
```bash
tw-news-yahoo serve
```

## 資料庫位置

預設：`~/.tw-news-yahoo/news.db`

```bash
tw-news-yahoo list --db /path/to/custom.db
```

## 自動每日同步（crontab）

```bash
# 每天早上 7 點同步彰化縣長相關新聞
0 7 * * * /usr/local/bin/tw-news-yahoo sync --keyword 彰化縣長
```
