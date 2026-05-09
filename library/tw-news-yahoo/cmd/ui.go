package cmd

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
	"github.com/strongershen/tw-news-yahoo/internal/yahoo"
)

//go:embed ui.html
var uiHTML []byte

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "啟動本機 Web UI（瀏覽器操作介面）",
	RunE:  runUI,
}

var uiPort int

func init() {
	rootCmd.AddCommand(uiCmd)
	uiCmd.Flags().IntVar(&uiPort, "port", 8080, "監聽埠號")
}

func runUI(cmd *cobra.Command, args []string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/fetch", handleUIFetch)
	mux.HandleFunc("/api/sync", handleUISync)
	mux.HandleFunc("/api/articles", handleArticles)
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/timeline", handleTimeline)
	mux.HandleFunc("/api/export", handleUIExport)
	mux.HandleFunc("/api/stats", handleStats)

	addr := fmt.Sprintf("localhost:%d", uiPort)
	url := "http://" + addr
	fmt.Printf("Web UI 啟動於 %s\n按 Ctrl+C 停止\n", url)

	go func() {
		time.Sleep(300 * time.Millisecond)
		openBrowser(url)
	}()

	return http.ListenAndServe(addr, mux)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(uiHTML)
}

// sseStream sets SSE headers and streams messages produced by run.
func sseStream(w http.ResponseWriter, r *http.Request, run func(chan<- string)) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	progress := make(chan string, 200)
	go func() {
		defer close(progress)
		run(progress)
	}()

	ctx := r.Context()
	for {
		select {
		case msg, ok := <-progress:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func handleUIFetch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	county := q.Get("county")
	keyword := q.Get("keyword")
	all := q.Get("all") == "true"
	topicURL := q.Get("topic")
	if topicURL == "" {
		topicURL = "https://tw.news.yahoo.com/topic/2026election/"
	}
	sseStream(w, r, func(ch chan<- string) {
		uiFetchProgress(county, keyword, all, topicURL, ch)
	})
}

func handleUISync(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	county := q.Get("county")
	keyword := q.Get("keyword")
	since := q.Get("since")
	if since == "" {
		since = "last"
	}
	topicURL := q.Get("topic")
	if topicURL == "" {
		topicURL = "https://tw.news.yahoo.com/topic/2026election/"
	}
	sseStream(w, r, func(ch chan<- string) {
		uiSyncProgress(county, keyword, since, topicURL, ch)
	})
}

func handleArticles(w http.ResponseWriter, r *http.Request) {
	st, err := store.Open(dbPath)
	if err != nil {
		jsonErr(w, err)
		return
	}
	defer st.Close()

	q := r.URL.Query()
	f := store.Filter{
		Keyword: q.Get("keyword"),
		Limit:   intQ(q.Get("limit"), 50),
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.From = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			f.To = t.Add(24 * time.Hour)
		}
	}
	articles, err := st.List(f)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, toJSONSlice(articles))
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	st, err := store.Open(dbPath)
	if err != nil {
		jsonErr(w, err)
		return
	}
	defer st.Close()

	q := r.URL.Query().Get("q")
	limit := intQ(r.URL.Query().Get("limit"), 20)
	articles, err := st.Search(q, limit)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, toJSONSlice(articles))
}

func handleTimeline(w http.ResponseWriter, r *http.Request) {
	st, err := store.Open(dbPath)
	if err != nil {
		jsonErr(w, err)
		return
	}
	defer st.Close()

	loc := taipeiLoc()
	now := time.Now().In(loc)
	from := now.AddDate(0, 0, -30)
	to := now

	q := r.URL.Query()
	if v := q.Get("from"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			from = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			to = t
		}
	}

	daily, err := st.DailyCount(from.UTC(), to.Add(24*time.Hour).UTC())
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, daily)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := store.Open(dbPath)
	if err != nil {
		jsonErr(w, err)
		return
	}
	defer st.Close()

	last := st.LastSyncTime()
	lastStr := ""
	if !last.IsZero() {
		lastStr = last.In(taipeiLoc()).Format("2006-01-02 15:04:05")
	}
	jsonOK(w, map[string]any{
		"count":    st.Count(),
		"lastSync": lastStr,
	})
}

func handleUIExport(w http.ResponseWriter, r *http.Request) {
	st, err := store.Open(dbPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer st.Close()

	q := r.URL.Query()
	format := q.Get("format")
	keyword := q.Get("keyword")
	limit := intQ(q.Get("limit"), 0)

	articles, err := st.List(store.Filter{Keyword: keyword, Limit: limit})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch strings.ToLower(format) {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="articles.csv"`)
		exportCSV(w, articles)
	case "markdown", "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="articles.md"`)
		exportMarkdown(w, articles)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="articles.json"`)
		exportJSON(w, articles)
	}
}

// ---- progress workers ----

func uiFetchProgress(county, keyword string, all bool, topicURL string, ch chan<- string) {
	st, err := store.Open(dbPath)
	if err != nil {
		ch <- "ERROR: " + err.Error()
		return
	}
	defer st.Close()

	client := yahoo.New()
	ch <- "正在抓取 topic 頁：" + topicURL
	links, err := client.FetchTopicLinks(topicURL)
	if err != nil {
		ch <- "ERROR: " + err.Error()
		return
	}
	ch <- fmt.Sprintf("找到 %d 篇文章連結", len(links))

	filters := uiBuildFilters(county, keyword, all, ch)

	var saved, skipped, filtered int
	for i, lnk := range links {
		ch <- fmt.Sprintf("[%d/%d] %s", i+1, len(links), lnk.URL)
		meta, err := client.FetchArticleMeta(lnk.URL)
		if err != nil {
			ch <- fmt.Sprintf("  ⚠ 跳過：%v", err)
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
			uuid = lnk.Path
		}
		if err := st.Upsert(toStoreArticle(uuid, meta)); err != nil {
			ch <- fmt.Sprintf("  ⚠ 儲存失敗：%v", err)
		} else {
			saved++
			ch <- "  ✓ " + meta.Headline
		}
		time.Sleep(client.Delay)
	}
	st.SetLastSyncTime(time.Now())
	ch <- fmt.Sprintf("完成：儲存 %d 篇，過濾 %d 篇，跳過 %d 篇（錯誤）", saved, filtered, skipped)
	ch <- fmt.Sprintf("資料庫共 %d 篇文章", st.Count())
	ch <- "DONE"
}

func uiSyncProgress(county, keyword, since, topicURL string, ch chan<- string) {
	st, err := store.Open(dbPath)
	if err != nil {
		ch <- "ERROR: " + err.Error()
		return
	}
	defer st.Close()

	var sinceTime time.Time
	switch since {
	case "last":
		sinceTime = st.LastSyncTime()
		if sinceTime.IsZero() {
			ch <- "尚無同步記錄，抓取全部文章"
		} else {
			ch <- "上次同步：" + sinceTime.In(taipeiLoc()).Format("2006-01-02 15:04:05")
		}
	default:
		for _, f := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
			if t, err := time.Parse(f, since); err == nil {
				sinceTime = t
				break
			}
		}
	}

	client := yahoo.New()
	ch <- "正在抓取 topic 頁：" + topicURL
	links, err := client.FetchTopicLinks(topicURL)
	if err != nil {
		ch <- "ERROR: " + err.Error()
		return
	}
	ch <- fmt.Sprintf("找到 %d 篇文章連結", len(links))

	filters := uiBuildFilters(county, keyword, false, ch)

	var newCount, existCount, filteredCount, errCount int
	for i, lnk := range links {
		if st.ExistsByURL(lnk.URL) {
			ch <- fmt.Sprintf("[%d/%d] 已存在，跳過", i+1, len(links))
			existCount++
			continue
		}
		ch <- fmt.Sprintf("[%d/%d] %s", i+1, len(links), lnk.URL)
		meta, err := client.FetchArticleMeta(lnk.URL)
		if err != nil {
			ch <- fmt.Sprintf("  ⚠ 錯誤：%v", err)
			errCount++
			time.Sleep(client.Delay)
			continue
		}
		if !sinceTime.IsZero() && !meta.DatePublished.IsZero() && meta.DatePublished.Before(sinceTime) {
			ch <- fmt.Sprintf("  太舊（%s），跳過", meta.DatePublished.Format("2006-01-02"))
			filteredCount++
			time.Sleep(client.Delay)
			continue
		}
		if len(filters) > 0 && !meta.MatchesKeywords(filters) {
			filteredCount++
			time.Sleep(client.Delay)
			continue
		}
		uuid := meta.UUID
		if uuid == "" {
			uuid = lnk.Path
		}
		if err := st.Upsert(toStoreArticle(uuid, meta)); err != nil {
			ch <- fmt.Sprintf("  ⚠ 儲存失敗：%v", err)
		} else {
			newCount++
			ch <- "  ✓ 新文章：" + meta.Headline
		}
		time.Sleep(client.Delay)
	}
	st.SetLastSyncTime(time.Now())
	ch <- fmt.Sprintf("同步完成：新增 %d 篇，已存在 %d 篇，過濾 %d 篇，錯誤 %d 篇", newCount, existCount, filteredCount, errCount)
	ch <- fmt.Sprintf("資料庫共 %d 篇文章", st.Count())
	ch <- "DONE"
}

func uiBuildFilters(county, keyword string, all bool, ch chan<- string) []string {
	if all {
		return nil
	}
	var filters []string
	if county != "" {
		if profile, ok := countyProfiles[county]; ok {
			filters = append(filters, profile.candidates...)
			ch <- fmt.Sprintf("縣市：%s（關鍵字：%v）", profile.zhName, profile.candidates)
		}
	}
	for _, k := range splitKeywords(keyword) {
		filters = append(filters, k)
	}
	if county == "" && keyword == "" {
		filters = []string{"彰化"}
		ch <- "（未指定縣市或關鍵字，預設過濾「彰化」）"
	}
	return filters
}

// ---- helpers ----

type articleRow struct {
	UUID          string   `json:"uuid"`
	URL           string   `json:"url"`
	Headline      string   `json:"headline"`
	Description   string   `json:"description"`
	Keywords      []string `json:"keywords"`
	Provider      string   `json:"provider"`
	DatePublished string   `json:"date_published"`
}

func toJSONSlice(articles []*store.Article) []articleRow {
	out := make([]articleRow, 0, len(articles))
	loc := taipeiLoc()
	for _, a := range articles {
		out = append(out, articleRow{
			UUID:          a.UUID,
			URL:           a.URL,
			Headline:      a.Headline,
			Description:   a.Description,
			Keywords:      a.Keywords,
			Provider:      a.Provider,
			DatePublished: a.DatePublished.In(loc).Format("2006-01-02 15:04"),
		})
	}
	return out
}

func toStoreArticle(uuid string, m *yahoo.ArticleMeta) *store.Article {
	return &store.Article{
		UUID:          uuid,
		URL:           m.URL,
		Headline:      m.Headline,
		Description:   m.Description,
		Keywords:      m.Keywords,
		Entities:      m.Entities,
		Provider:      m.Provider,
		DatePublished: m.DatePublished,
		DateModified:  m.DateModified,
		FetchedAt:     time.Now().UTC(),
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func intQ(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
