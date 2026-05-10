# printing-press 專案規則

## Repo 結構

`library/tw-news-yahoo/` 是 git submodule，指向獨立 repo：

| Repo | URL | 內容 |
|------|-----|------|
| printing-press | https://github.com/StrongerShen/printing-press | 工具集根目錄 |
| 2026election | https://github.com/StrongerShen/2026election | `library/tw-news-yahoo/` 的獨立 repo |

## 日常工作流

### 修改程式碼後（在 `library/tw-news-yahoo/` 內）

```bash
cd library/tw-news-yahoo
git add <files>
git commit -m "..."
git push origin main          # 推到 2026election
```

### 更新 printing-press 的 submodule 指標

```bash
cd ~/printing-press
git add library/tw-news-yahoo
git commit -m "update submodule: <描述>"
git push origin main
```

### 換台電腦後同步

```bash
git pull
git submodule update --remote  # 同步 submodule 到最新
```

### 第一次 clone（新電腦）

```bash
git clone --recurse-submodules https://github.com/StrongerShen/printing-press.git
```

## README 規則

功能有異動時，**必須同步更新**對應的 README：

- `~/printing-press/README.md` — 工具集總覽
- `library/tw-news-yahoo/README.md` — tw-news-yahoo 詳細說明

不要等用戶提醒，改完程式碼就一併更新。

## Build 規則（tw-news-yahoo）

```bash
cd library/tw-news-yahoo

# macOS binary
go build -o tw-news-yahoo .

# Windows binary（cross-compile）
GOOS=windows GOARCH=amd64 go build -o tw-news-yahoo.exe .
```

兩個 binary 都要一起 build 並 commit。
