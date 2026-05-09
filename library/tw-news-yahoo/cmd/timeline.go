package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "顯示文章發布頻率的 ASCII 趨勢圖",
	RunE:  runTimeline,
}

var (
	timelineFrom  string
	timelineTo    string
	timelineWidth int
)

func init() {
	rootCmd.AddCommand(timelineCmd)
	timelineCmd.Flags().StringVar(&timelineFrom, "from", "", "起始日期 YYYY-MM-DD（預設：30 天前）")
	timelineCmd.Flags().StringVar(&timelineTo, "to", "", "結束日期 YYYY-MM-DD（預設：今天）")
	timelineCmd.Flags().IntVar(&timelineWidth, "width", 40, "長條圖最大寬度")
}

func runTimeline(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	loc := taipeiLoc()
	now := time.Now().In(loc)

	from := now.AddDate(0, 0, -30)
	to := now

	if timelineFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", timelineFrom, loc); err == nil {
			from = t
		} else {
			return fmt.Errorf("無法解析 --from：%s", timelineFrom)
		}
	}
	if timelineTo != "" {
		if t, err := time.ParseInLocation("2006-01-02", timelineTo, loc); err == nil {
			to = t.Add(24 * time.Hour)
		} else {
			return fmt.Errorf("無法解析 --to：%s", timelineTo)
		}
	}

	daily, err := st.DailyCount(from.UTC(), to.UTC())
	if err != nil {
		return err
	}
	if len(daily) == 0 {
		fmt.Println("指定時間範圍內無資料，請先執行 fetch 或 sync")
		return nil
	}

	// sort dates
	var dates []string
	for d := range daily {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	maxCount := 0
	for _, n := range daily {
		if n > maxCount {
			maxCount = n
		}
	}

	fmt.Printf("\n文章發布頻率（%s ～ %s）\n\n",
		from.Format("2006-01-02"), to.Format("2006-01-02"))

	for _, d := range dates {
		n := daily[d]
		barLen := 0
		if maxCount > 0 {
			barLen = n * timelineWidth / maxCount
		}
		bar := strings.Repeat("█", barLen)
		fmt.Printf("  %s │%s %d\n", d, bar, n)
	}
	fmt.Println()
	return nil
}
