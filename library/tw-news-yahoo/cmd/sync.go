package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
	"github.com/strongershen/tw-news-yahoo/internal/yahoo"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "增量同步：只抓比上次更新時間更新的文章",
	RunE:  runSync,
}

var (
	syncTopicURL string
	syncKeywords string
	syncSince    string
	syncAll      bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringVar(&syncTopicURL, "topic", "https://tw.news.yahoo.com/topic/2026election/", "Topic 頁 URL")
	syncCmd.Flags().StringVar(&syncKeywords, "keyword", "彰化", "過濾關鍵字（逗號分隔）")
	syncCmd.Flags().StringVar(&syncSince, "since", "last", "起始時間：'last'（上次同步）、RFC3339 日期或 YYYY-MM-DD")
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "不做關鍵字過濾")
}

func runSync(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return fmt.Errorf("開啟資料庫：%w", err)
	}
	defer st.Close()

	var since time.Time
	switch syncSince {
	case "last":
		since = st.LastSyncTime()
		if since.IsZero() {
			fmt.Println("尚無同步記錄，抓取全部文章")
		} else {
			fmt.Printf("上次同步：%s（台北時間）\n", since.In(taipeiLoc()).Format("2006-01-02 15:04:05"))
		}
	case "":
		// no filter
	default:
		formats := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
		for _, f := range formats {
			if t, err := time.Parse(f, syncSince); err == nil {
				since = t
				break
			}
		}
		if since.IsZero() {
			return fmt.Errorf("無法解析 --since 時間：%s", syncSince)
		}
	}

	client := yahoo.New()
	fmt.Printf("正在抓取 topic 頁：%s\n", syncTopicURL)
	links, err := client.FetchTopicLinks(syncTopicURL)
	if err != nil {
		return err
	}
	fmt.Printf("找到 %d 篇文章連結\n", len(links))

	filters := splitKeywords(syncKeywords)
	if syncAll {
		filters = nil
	}

	var newCount, existCount, filteredCount, errCount int
	for i, lnk := range links {
		fmt.Printf("[%d/%d] ", i+1, len(links))

		if st.ExistsByURL(lnk.URL) {
			fmt.Printf("已存在，跳過\n")
			existCount++
			continue
		}

		meta, err := client.FetchArticleMeta(lnk.URL)
		if err != nil {
			fmt.Printf("⚠ 錯誤：%v\n", err)
			errCount++
			time.Sleep(client.Delay)
			continue
		}

		if !since.IsZero() && !meta.DatePublished.IsZero() && meta.DatePublished.Before(since) {
			fmt.Printf("太舊（%s），跳過\n", meta.DatePublished.Format("2006-01-02"))
			filteredCount++
			time.Sleep(client.Delay)
			continue
		}

		if len(filters) > 0 && !meta.MatchesKeywords(filters) {
			fmt.Printf("關鍵字不符，跳過\n")
			filteredCount++
			time.Sleep(client.Delay)
			continue
		}

		uuid := meta.UUID
		if uuid == "" {
			uuid = pathToID(lnk.Path)
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
			fmt.Printf("⚠ 儲存失敗：%v\n", err)
		} else {
			fmt.Printf("✓ 新文章：%s\n", meta.Headline)
			newCount++
		}
		time.Sleep(client.Delay)
	}

	st.SetLastSyncTime(time.Now())
	fmt.Printf("\n同步完成：新增 %d 篇，已存在 %d 篇，過濾 %d 篇，錯誤 %d 篇\n",
		newCount, existCount, filteredCount, errCount)
	fmt.Printf("資料庫共 %d 篇文章\n", st.Count())
	return nil
}

func pathToID(path string) string {
	return path
}
