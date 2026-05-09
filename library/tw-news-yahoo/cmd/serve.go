package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strongershen/tw-news-yahoo/internal/store"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "啟動 MCP stdio server（供 AI Agent 整合使用）",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

// Minimal MCP stdio server (JSON-RPC 2.0 over stdin/stdout)
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *mcpError `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func runServe(cmd *cobra.Command, args []string) error {
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()

	enc := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)

	// Announce capabilities on startup
	enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params": map[string]any{
			"serverInfo": map[string]any{
				"name":    "tw-news-yahoo",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		},
	})

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req mcpRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}

		var resp mcpResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "initialize":
			resp.Result = map[string]any{
				"serverInfo": map[string]any{"name": "tw-news-yahoo", "version": "1.0.0"},
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"protocolVersion": "2024-11-05",
			}

		case "tools/list":
			resp.Result = map[string]any{
				"tools": []map[string]any{
					{
						"name":        "search",
						"description": "搜尋 Yahoo 台灣選舉新聞（FTS5 全文搜尋）",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query": map[string]any{"type": "string", "description": "搜尋關鍵字"},
								"limit": map[string]any{"type": "integer", "description": "最多幾筆", "default": 10},
							},
							"required": []string{"query"},
						},
					},
					{
						"name":        "list",
						"description": "列出本機資料庫中的選舉新聞",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"keyword": map[string]any{"type": "string"},
								"limit":   map[string]any{"type": "integer", "default": 20},
							},
						},
					},
				},
			}

		case "tools/call":
			var params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			json.Unmarshal(req.Params, &params)

			switch params.Name {
			case "search":
				var a struct {
					Query string `json:"query"`
					Limit int    `json:"limit"`
				}
				json.Unmarshal(params.Arguments, &a)
				if a.Limit == 0 {
					a.Limit = 10
				}
				articles, err := st.Search(a.Query, a.Limit)
				if err != nil {
					resp.Error = &mcpError{Code: -32000, Message: err.Error()}
				} else {
					resp.Result = map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": formatArticlesText(articles)},
						},
					}
				}

			case "list":
				var a struct {
					Keyword string `json:"keyword"`
					Limit   int    `json:"limit"`
				}
				json.Unmarshal(params.Arguments, &a)
				if a.Limit == 0 {
					a.Limit = 20
				}
				articles, err := st.List(store.Filter{Keyword: a.Keyword, Limit: a.Limit})
				if err != nil {
					resp.Error = &mcpError{Code: -32000, Message: err.Error()}
				} else {
					resp.Result = map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": formatArticlesText(articles)},
						},
					}
				}

			default:
				resp.Error = &mcpError{Code: -32601, Message: fmt.Sprintf("unknown tool: %s", params.Name)}
			}

		default:
			resp.Error = &mcpError{Code: -32601, Message: "method not found"}
		}

		enc.Encode(resp)
	}
	return scanner.Err()
}

func formatArticlesText(articles []*store.Article) string {
	if len(articles) == 0 {
		return "找不到符合條件的文章"
	}
	var sb strings.Builder
	for _, a := range articles {
		fmt.Fprintf(&sb, "【%s】%s\n", a.DatePublished.In(taipeiLoc()).Format("2006-01-02"), a.Headline)
		if a.Description != "" {
			fmt.Fprintf(&sb, "  %s\n", a.Description)
		}
		fmt.Fprintf(&sb, "  %s\n\n", a.URL)
	}
	return sb.String()
}
