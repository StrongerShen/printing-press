package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
	"github.com/strongershen/tw-news-yahoo/internal/yahoo"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "抓取 topic 頁文章並存入資料庫",
	Long:  `從 Yahoo 新聞 2026 選舉 topic 頁抓取最新文章，依關鍵字過濾後存入本機 SQLite。`,
	RunE:  runFetch,
}

var (
	fetchTopicURL string
	fetchKeywords string
	fetchCounty   string
	fetchAll      bool
)

func init() {
	rootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().StringVar(&fetchTopicURL, "topic", "https://tw.news.yahoo.com/topic/2026election/", "Topic 頁 URL")
	fetchCmd.Flags().StringVar(&fetchKeywords, "keyword", "", "過濾關鍵字（逗號分隔）")
	fetchCmd.Flags().StringVar(&fetchCounty, "county", "", "縣市代碼（changhua/taipei/taichung/kaohsiung/tainan/taoyuan）")
	fetchCmd.Flags().BoolVar(&fetchAll, "all", false, "儲存所有文章，不做關鍵字過濾")
}

func runFetch(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return fmt.Errorf("開啟資料庫：%w", err)
	}
	defer st.Close()

	client := yahoo.New()

	fmt.Printf("正在抓取 topic 頁：%s\n", fetchTopicURL)
	links, err := client.FetchTopicLinks(fetchTopicURL)
	if err != nil {
		return fmt.Errorf("抓取 topic：%w", err)
	}
	fmt.Printf("找到 %d 篇文章連結\n", len(links))

	var filters []string
	if !fetchAll {
		// --county 展開為預設關鍵字包
		if fetchCounty != "" {
			profile, ok := countyProfiles[fetchCounty]
			if !ok {
				return fmt.Errorf("不支援的縣市代碼：%s\n可用：changhua, taipei, taichung, kaohsiung, tainan, taoyuan", fetchCounty)
			}
			filters = append(filters, profile.candidates...)
			fmt.Printf("縣市：%s（關鍵字：%v）\n", profile.zhName, profile.candidates)
		}
		// --keyword 額外附加
		for _, k := range splitKeywords(fetchKeywords) {
			filters = append(filters, k)
		}
		// 若兩者皆未指定，預設過濾「彰化」
		if fetchCounty == "" && fetchKeywords == "" {
			filters = []string{"彰化"}
			fmt.Println("（未指定縣市或關鍵字，預設過濾「彰化」）")
		}
	}

	var saved, skipped, filtered int
	for i, lnk := range links {
		fmt.Printf("[%d/%d] 查詢：%s\n", i+1, len(links), lnk.URL)
		meta, err := client.FetchArticleMeta(lnk.URL)
		if err != nil {
			fmt.Printf("  ⚠ 跳過（%v）\n", err)
			skipped++
			time.Sleep(client.Delay)
			continue
		}
		if len(filters) > 0 && !meta.MatchesKeywords(filters) {
			filtered++
			time.Sleep(client.Delay)
			continue
		}

		uuid := meta.UUID
		if uuid == "" {
			uuid = lnk.Path // fallback key
		}

		art := &store.Article{
			UUID:          uuid,
			URL:           meta.URL,
			Headline:      meta.Headline,
			Description:   meta.Description,
			Keywords:      meta.Keywords,
			Entities:      meta.Entities,
			Provider:      meta.Provider,
			DatePublished: meta.DatePublished,
			DateModified:  meta.DateModified,
			FetchedAt:     time.Now().UTC(),
		}
		if err := st.Upsert(art); err != nil {
			fmt.Printf("  ⚠ 儲存失敗：%v\n", err)
		} else {
			saved++
			fmt.Printf("  ✓ 儲存：%s\n", meta.Headline)
		}
		time.Sleep(client.Delay)
	}

	st.SetLastSyncTime(time.Now())
	fmt.Printf("\n完成：儲存 %d 篇，過濾 %d 篇，跳過 %d 篇（錯誤）\n", saved, filtered, skipped)
	fmt.Printf("資料庫共 %d 篇文章\n", st.Count())
	return nil
}
