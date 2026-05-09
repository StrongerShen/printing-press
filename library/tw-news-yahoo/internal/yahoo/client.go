package yahoo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	topicBase = "https://tw.news.yahoo.com"
	caasBase  = "https://tw.news.yahoo.com/caas/content/article"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

var articleLinkRe = regexp.MustCompile(`href="(/[^"]*-\d+\.html)"`)

type ArticleLink struct {
	Path string
	URL  string
}

type ArticleMeta struct {
	UUID          string
	URL           string
	Headline      string
	Description   string
	Keywords      []string
	DatePublished time.Time
	DateModified  time.Time
	Provider      string
	Entities      []string
}

type Client struct {
	http    *http.Client
	Delay   time.Duration
}

func New() *Client {
	return &Client{
		http:  &http.Client{Timeout: 20 * time.Second},
		Delay: 200 * time.Millisecond,
	}
}

func (c *Client) get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(resp.Body)
}

// FetchTopicLinks scrapes the topic page and returns unique article links.
func (c *Client) FetchTopicLinks(topicURL string) ([]ArticleLink, error) {
	body, err := c.get(topicURL)
	if err != nil {
		return nil, fmt.Errorf("fetch topic page: %w", err)
	}
	html := string(body)
	matches := articleLinkRe.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	var out []ArticleLink
	for _, m := range matches {
		path := m[1]
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, ArticleLink{
			Path: path,
			URL:  topicBase + path,
		})
	}
	return out, nil
}

// caasResponse mirrors the CaaS JSON structure we need.
type caasResponse struct {
	Items []struct {
		Schema struct {
			Default *schemaDefault `json:"default"`
		} `json:"schema"`
		Data struct {
			PartnerData *partnerData `json:"partnerData"`
		} `json:"data"`
	} `json:"items"`
}

type schemaDefault struct {
	Headline      string   `json:"headline"`
	Description   string   `json:"description"`
	Keywords      []string `json:"keywords"`
	DatePublished string   `json:"datePublished"`
	DateModified  string   `json:"dateModified"`
}

type partnerData struct {
	UUID        string `json:"uuid"`
	URL         string `json:"url"`
	PublishDate string `json:"publishDate"`
	Keywords    string `json:"keywords"`
	Attribution struct {
		Provider struct {
			Name string `json:"name"`
		} `json:"provider"`
	} `json:"attribution"`
	Entities []struct {
		Label string `json:"label"`
	} `json:"entities"`
}

// FetchArticleMeta calls the CaaS API for one article URL.
func (c *Client) FetchArticleMeta(articleURL string) (*ArticleMeta, error) {
	caasURL := caasBase + "?url=" + url.QueryEscape(articleURL)
	body, err := c.get(caasURL)
	if err != nil {
		return nil, fmt.Errorf("caas fetch: %w", err)
	}
	var resp caasResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("caas parse: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("caas: empty items for %s", articleURL)
	}
	item := resp.Items[0]

	meta := &ArticleMeta{}

	if sd := item.Schema.Default; sd != nil {
		meta.Headline = sd.Headline
		meta.Description = sd.Description
		meta.Keywords = sd.Keywords
		if t, err := time.Parse(time.RFC3339, sd.DatePublished); err == nil {
			meta.DatePublished = t
		} else if t, err := time.Parse("2006-01-02T15:04:05.000Z", sd.DatePublished); err == nil {
			meta.DatePublished = t
		}
		if t, err := time.Parse(time.RFC3339, sd.DateModified); err == nil {
			meta.DateModified = t
		} else if t, err := time.Parse("2006-01-02T15:04:05.000Z", sd.DateModified); err == nil {
			meta.DateModified = t
		}
	}

	if pd := item.Data.PartnerData; pd != nil {
		meta.UUID = pd.UUID
		meta.URL = pd.URL
		meta.Provider = pd.Attribution.Provider.Name
		for _, e := range pd.Entities {
			if e.Label != "" {
				meta.Entities = append(meta.Entities, e.Label)
			}
		}
		// Fallback: merge partnerData keywords (dedup)
		if pd.Keywords != "" {
			existing := map[string]bool{}
			for _, k := range meta.Keywords {
				existing[k] = true
			}
			for _, k := range strings.Split(pd.Keywords, ",") {
				k = strings.TrimSpace(k)
				if k != "" && !existing[k] {
					meta.Keywords = append(meta.Keywords, k)
					existing[k] = true
				}
			}
		}
		// Use partnerData URL as canonical if schema didn't set it
		if meta.URL == "" {
			meta.URL = articleURL
		}
		// Fallback publishDate
		if meta.DatePublished.IsZero() && pd.PublishDate != "" {
			if t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", pd.PublishDate); err == nil {
				meta.DatePublished = t
			}
		}
	}

	if meta.URL == "" {
		meta.URL = articleURL
	}

	return meta, nil
}

// MatchesKeywords reports whether any of the filter keywords appear in headline, description, or keywords.
func (m *ArticleMeta) MatchesKeywords(filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	haystack := strings.ToLower(m.Headline + " " + m.Description + " " + strings.Join(m.Keywords, " ") + " " + strings.Join(m.Entities, " "))
	for _, f := range filters {
		if strings.Contains(haystack, strings.ToLower(f)) {
			return true
		}
	}
	return false
}
