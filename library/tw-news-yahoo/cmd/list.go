package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出本機已存文章",
	RunE:  runList,
}

var (
	listKeyword string
	listFrom    string
	listTo      string
	listLimit   int
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listKeyword, "keyword", "", "依關鍵字過濾（LIKE 比對）")
	listCmd.Flags().StringVar(&listFrom, "from", "", "起始日期（YYYY-MM-DD）")
	listCmd.Flags().StringVar(&listTo, "to", "", "結束日期（YYYY-MM-DD）")
	listCmd.Flags().IntVar(&listLimit, "limit", 50, "最多顯示幾筆")
}

func runList(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	f := store.Filter{
		Keyword: listKeyword,
		Limit:   listLimit,
	}

	if listFrom != "" {
		t, err := time.Parse("2006-01-02", listFrom)
		if err != nil {
			return fmt.Errorf("無法解析 --from：%s", listFrom)
		}
		f.From = t
	}
	if listTo != "" {
		t, err := time.Parse("2006-01-02", listTo)
		if err != nil {
			return fmt.Errorf("無法解析 --to：%s", listTo)
		}
		f.To = t.Add(24 * time.Hour)
	}

	articles, err := st.List(f)
	if err != nil {
		return err
	}

	if len(articles) == 0 {
		if !jsonOut {
			fmt.Println("資料庫中找不到符合條件的文章，請先執行 fetch 或 sync")
		} else {
			fmt.Println("[]")
		}
		return nil
	}
	if !jsonOut {
		fmt.Printf("共 %d 篇文章：\n\n", len(articles))
	}
	printArticles(articles)
	return nil
}
