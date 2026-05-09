package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
)

// countyProfiles maps county names to preset candidate keywords.
var countyProfiles = map[string]struct {
	zhName     string
	candidates []string
}{
	"changhua": {"彰化", []string{"彰化縣長", "王惠美", "陳世凱", "彰化選舉", "彰化縣"}},
	"taipei":   {"台北", []string{"台北市長", "蔣萬安", "台北市"}},
	"taichung": {"台中", []string{"台中市長", "盧秀燕", "台中市"}},
	"kaohsiung": {"高雄", []string{"高雄市長", "陳其邁", "高雄市"}},
	"tainan":   {"台南", []string{"台南市長", "黃偉哲", "台南市"}},
	"taoyuan":  {"桃園", []string{"桃園市長", "桃園市"}},
}

var candidatesCmd = &cobra.Command{
	Use:   "candidates",
	Short: "用預設縣市關鍵字快速查詢候選人新聞",
	Long: `依縣市查詢候選人相關新聞。可用縣市代碼：
  changhua  彰化縣
  taipei    台北市
  taichung  台中市
  kaohsiung 高雄市
  tainan    台南市
  taoyuan   桃園市`,
	RunE: runCandidates,
}

var (
	candidateCounty string
	candidateLimit  int
)

func init() {
	rootCmd.AddCommand(candidatesCmd)
	candidatesCmd.Flags().StringVar(&candidateCounty, "county", "changhua", "縣市代碼（changhua/taipei/taichung/kaohsiung/tainan/taoyuan）")
	candidatesCmd.Flags().IntVar(&candidateLimit, "limit", 30, "最多顯示幾筆")
}

func runCandidates(cmd *cobra.Command, args []string) error {
	profile, ok := countyProfiles[strings.ToLower(candidateCounty)]
	if !ok {
		return fmt.Errorf("不支援的縣市代碼：%s\n可用：changhua, taipei, taichung, kaohsiung, tainan, taoyuan", candidateCounty)
	}

	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	// Search using first keyword (FTS5), then filter others with LIKE in List
	var allResults []*store.Article
	seen := map[string]bool{}

	for _, kw := range profile.candidates {
		arts, err := st.Search(kw, candidateLimit)
		if err != nil {
			continue
		}
		for _, a := range arts {
			if !seen[a.UUID] {
				seen[a.UUID] = true
				allResults = append(allResults, a)
			}
		}
	}

	if len(allResults) == 0 {
		fmt.Printf("找不到「%s」相關文章，請先執行：\n", profile.zhName)
		fmt.Printf("  tw-news-yahoo fetch --keyword %s\n", profile.candidates[0])
		return nil
	}

	fmt.Printf("【%s】候選人相關文章（共 %d 篇）：\n\n", profile.zhName, len(allResults))
	if len(allResults) > candidateLimit {
		allResults = allResults[:candidateLimit]
	}
	printArticles(allResults)
	return nil
}
