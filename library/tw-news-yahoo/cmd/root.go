package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
)

var (
	dbPath   string
	jsonOut  bool
)

var rootCmd = &cobra.Command{
	Use:   "tw-news-yahoo",
	Short: "Yahoo 台灣新聞 CLI — 追蹤 2026 縣市長選情",
	Long: `tw-news-yahoo 從 Yahoo 台灣新聞抓取選舉相關文章，
支援關鍵字過濾（如「彰化縣長」）、本機 SQLite 儲存與離線搜尋。`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	defaultDB := filepath.Join(os.Getenv("HOME"), ".tw-news-yahoo", "news.db")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDB, "SQLite 資料庫路徑")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "以 JSON 格式輸出")
}

func openStore() (*store.Store, error) {
	return store.Open(dbPath)
}

// printArticles prints articles as table or JSON.
func printArticles(articles []*store.Article) {
	if jsonOut {
		type row struct {
			UUID          string   `json:"uuid"`
			URL           string   `json:"url"`
			Headline      string   `json:"headline"`
			Description   string   `json:"description"`
			Keywords      []string `json:"keywords"`
			Entities      []string `json:"entities"`
			Provider      string   `json:"provider"`
			DatePublished string   `json:"date_published"`
		}
		var rows []row
		for _, a := range articles {
			rows = append(rows, row{
				UUID:          a.UUID,
				URL:           a.URL,
				Headline:      a.Headline,
				Description:   a.Description,
				Keywords:      a.Keywords,
				Entities:      a.Entities,
				Provider:      a.Provider,
				DatePublished: a.DatePublished.Format(time.RFC3339),
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(rows)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "日期\t來源\t標題\tURL")
	fmt.Fprintln(w, "----\t----\t----\t---")
	for _, a := range articles {
		date := a.DatePublished.In(taipeiLoc()).Format("2006-01-02 15:04")
		title := a.Headline
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:38]) + "…"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", date, a.Provider, title, a.URL)
	}
	w.Flush()
}

func taipeiLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return time.UTC
	}
	return loc
}

func splitKeywords(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}
