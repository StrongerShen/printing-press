package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <關鍵字>",
	Short: "對本機 SQLite 做 FTS5 全文搜尋",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSearch,
}

var searchLimit int

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "最多顯示幾筆")
}

func runSearch(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	query := args[0]
	articles, err := st.Search(query, searchLimit)
	if err != nil {
		return fmt.Errorf("搜尋失敗：%w", err)
	}
	if len(articles) == 0 {
		if !jsonOut {
			fmt.Printf("找不到符合「%s」的文章\n", query)
		} else {
			fmt.Println("[]")
		}
		return nil
	}
	if !jsonOut {
		fmt.Printf("搜尋「%s」，找到 %d 篇：\n\n", query, len(articles))
	}
	printArticles(articles)
	return nil
}
