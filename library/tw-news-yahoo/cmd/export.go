package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "將資料庫文章匯出為 JSON、CSV 或 Markdown",
	RunE:  runExport,
}

var (
	exportFormat  string
	exportOutput  string
	exportKeyword string
	exportLimit   int
)

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVar(&exportFormat, "format", "json", "格式：json、csv、markdown")
	exportCmd.Flags().StringVar(&exportOutput, "output", "", "輸出檔案路徑（預設：stdout）")
	exportCmd.Flags().StringVar(&exportKeyword, "keyword", "", "依關鍵字過濾")
	exportCmd.Flags().IntVar(&exportLimit, "limit", 0, "最多幾筆（0 表示全部）")
}

func runExport(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	articles, err := st.List(store.Filter{Keyword: exportKeyword, Limit: exportLimit})
	if err != nil {
		return err
	}
	if len(articles) == 0 {
		fmt.Fprintln(os.Stderr, "無符合條件的文章")
		return nil
	}

	out := os.Stdout
	if exportOutput != "" {
		f, err := os.Create(exportOutput)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	switch strings.ToLower(exportFormat) {
	case "json":
		return exportJSON(out, articles)
	case "csv":
		return exportCSV(out, articles)
	case "markdown", "md":
		return exportMarkdown(out, articles)
	default:
		return fmt.Errorf("不支援的格式：%s（可用：json, csv, markdown）", exportFormat)
	}
}

func exportJSON(f io.Writer, articles []*store.Article) error {
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
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func exportCSV(f io.Writer, articles []*store.Article) error {
	w := csv.NewWriter(f)
	w.Write([]string{"uuid", "date_published", "provider", "headline", "description", "keywords", "url"})
	for _, a := range articles {
		w.Write([]string{
			a.UUID,
			a.DatePublished.Format("2006-01-02 15:04:05"),
			a.Provider,
			a.Headline,
			a.Description,
			strings.Join(a.Keywords, ";"),
			a.URL,
		})
	}
	w.Flush()
	return w.Error()
}

func exportMarkdown(f io.Writer, articles []*store.Article) error {
	fmt.Fprintf(f, "# Yahoo 台灣選舉新聞匯出\n\n")
	fmt.Fprintf(f, "匯出時間：%s  共 %d 篇\n\n", time.Now().In(taipeiLoc()).Format("2006-01-02 15:04"), len(articles))
	for _, a := range articles {
		date := a.DatePublished.In(taipeiLoc()).Format("2006-01-02 15:04")
		fmt.Fprintf(f, "## %s\n\n", a.Headline)
		fmt.Fprintf(f, "**日期：** %s  **來源：** %s\n\n", date, a.Provider)
		if a.Description != "" {
			fmt.Fprintf(f, "%s\n\n", a.Description)
		}
		if len(a.Keywords) > 0 {
			fmt.Fprintf(f, "**關鍵字：** %s\n\n", strings.Join(a.Keywords, "、"))
		}
		fmt.Fprintf(f, "**連結：** [%s](%s)\n\n---\n\n", a.URL, a.URL)
	}
	return nil
}
