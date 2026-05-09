package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
	"github.com/strongershen/tw-news-yahoo/internal/yahoo"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "持續輪詢 topic 頁，有新文章時發出通知",
	RunE:  runWatch,
}

var (
	watchTopicURL string
	watchKeywords string
	watchInterval time.Duration
	watchNoNotify bool
)

func init() {
	rootCmd.AddCommand(watchCmd)
	watchCmd.Flags().StringVar(&watchTopicURL, "topic", "https://tw.news.yahoo.com/topic/2026election/", "Topic 頁 URL")
	watchCmd.Flags().StringVar(&watchKeywords, "keyword", "彰化", "過濾關鍵字（逗號分隔）")
	watchCmd.Flags().DurationVar(&watchInterval, "interval", time.Hour, "輪詢間隔（例：30m, 1h, 6h）")
	watchCmd.Flags().BoolVar(&watchNoNotify, "no-notify", false, "關閉桌面通知")
}

func runWatch(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	client := yahoo.New()
	filters := splitKeywords(watchKeywords)

	fmt.Printf("開始監控 %s\n", watchTopicURL)
	fmt.Printf("關鍵字：%v  間隔：%s\n", filters, watchInterval)
	fmt.Println("按 Ctrl+C 停止\n")

	for {
		fmt.Printf("[%s] 正在檢查新文章…\n", time.Now().In(taipeiLoc()).Format("15:04:05"))
		links, err := client.FetchTopicLinks(watchTopicURL)
		if err != nil {
			fmt.Printf("  ⚠ 抓取失敗：%v\n", err)
			time.Sleep(watchInterval)
			continue
		}

		var newArticles []*store.Article
		for _, lnk := range links {
			if st.Exists(lnk.Path) {
				continue
			}
			meta, err := client.FetchArticleMeta(lnk.URL)
			if err != nil {
				time.Sleep(client.Delay)
				continue
			}
			if len(filters) > 0 && !meta.MatchesKeywords(filters) {
				time.Sleep(client.Delay)
				continue
			}

			uuid := meta.UUID
			if uuid == "" {
				uuid = lnk.Path
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
			if err := st.Upsert(art); err == nil {
				newArticles = append(newArticles, art)
			}
			time.Sleep(client.Delay)
		}

		if len(newArticles) > 0 {
			fmt.Printf("  ✓ 發現 %d 篇新文章：\n", len(newArticles))
			for _, a := range newArticles {
				fmt.Printf("    • %s\n", a.Headline)
				fmt.Printf("      %s\n", a.URL)
			}
			if !watchNoNotify {
				notify(fmt.Sprintf("Yahoo 選舉新聞"), fmt.Sprintf("發現 %d 篇新文章", len(newArticles)))
			}
		} else {
			fmt.Println("  無新文章")
		}

		st.SetLastSyncTime(time.Now())
		fmt.Printf("  下次檢查：%s\n", time.Now().Add(watchInterval).In(taipeiLoc()).Format("15:04:05"))
		time.Sleep(watchInterval)
	}
}

func notify(title, body string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		exec.Command("osascript", "-e", script).Run()
	case "linux":
		exec.Command("notify-send", title, body).Run()
	}
}
