# printing-press

本機 CLI 工具集，由 [Printing Press](https://github.com/mvanhorn/cli-printing-press) 生成。

## 工具清單

| 工具 | 說明 | 位置 |
|------|------|------|
| [tw-news-yahoo](library/tw-news-yahoo/) | Yahoo 台灣新聞 CLI，追蹤 2026 縣市長選情 | `library/tw-news-yahoo/` |

## 快速開始

### tw-news-yahoo

抓取 Yahoo 台灣新聞 2026 選舉 topic 頁，依關鍵字過濾後存入本機 SQLite。

```bash
cd ~/printing-press/library/tw-news-yahoo

# 建置 macOS
go build -o tw-news-yahoo .

# 建置 Windows（cross-compile）
GOOS=windows GOARCH=amd64 go build -o tw-news-yahoo.exe .

# 首次抓取（預設過濾「彰化」）
./tw-news-yahoo fetch

# 依縣市代碼抓取
./tw-news-yahoo fetch --county changhua
./tw-news-yahoo fetch --county taipei

# 自訂關鍵字
./tw-news-yahoo fetch --keyword 彰化縣長,王惠美

# 搜尋
./tw-news-yahoo search 彰化縣長

# 增量同步
./tw-news-yahoo sync

# 增量同步（指定起始日期）
./tw-news-yahoo sync --since 2026-05-01

# 列出文章
./tw-news-yahoo list --keyword 彰化

# 瀏覽器 UI
./tw-news-yahoo ui
```

詳細用法請參閱 [library/tw-news-yahoo/README.md](library/tw-news-yahoo/README.md)。

## 目錄結構

```
~/printing-press/
├── README.md          # 本檔案
└── library/
    └── tw-news-yahoo/ # Yahoo 台灣選舉新聞 CLI
```
