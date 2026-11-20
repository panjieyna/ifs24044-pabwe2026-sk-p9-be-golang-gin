package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultSearchEndpoint adalah mesin pencari terbuka (tanpa API key) yang dipakai sebagai cadangan.
const DefaultSearchEndpoint = "https://id.wikipedia.org/w/api.php"

const (
	maxSources    = 5
	maxSnippetLen = 300
	userAgent     = "DelcomBlog/1.0 (educational project; AI blog generator)"
)

// SearchClient menelusuri sumber web untuk sebuah topik.
type SearchClient interface {
	Search(ctx context.Context, query string) ([]SearchSource, error)
}

// HTTPSearchClient memanggil SEARCH_ENGINE_ENDPOINT lalu jatuh ke Wikipedia bila endpoint utama gagal.
// Format respons yang dikenali: Wikipedia (query.search), DuckDuckGo (RelatedTopics), dan
// format generik {results|data|items|organic_results: [{title, url|link, snippet|content}]}.
type HTTPSearchClient struct {
	Endpoint         string
	FallbackEndpoint string
	APIKey           string
	HTTPClient       *http.Client
}

// NewSearchClient membuat HTTPSearchClient. API key hanya diteruskan bila host mesin pencari
// sama dengan host OPENAI_ENDPOINT, agar kunci tidak bocor ke layanan pihak ketiga.
func NewSearchClient(endpoint, openAIEndpoint, apiKey string) *HTTPSearchClient {
	endpoint = strings.TrimSpace(endpoint)
	key := ""
	if sameHost(endpoint, openAIEndpoint) {
		key = apiKey
	}
	return &HTTPSearchClient{
		Endpoint:         endpoint,
		FallbackEndpoint: DefaultSearchEndpoint,
		APIKey:           key,
		HTTPClient:       &http.Client{Timeout: 10 * time.Second},
	}
}

func sameHost(a, b string) bool {
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil || ua.Hostname() == "" {
		return false
	}
	return strings.EqualFold(ua.Hostname(), ub.Hostname())
}

// Search mengembalikan maksimal 5 sumber. Error hanya dikembalikan bila semua endpoint gagal.
func (s *HTTPSearchClient) Search(ctx context.Context, query string) ([]SearchSource, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchSource{}, nil
	}

	var firstErr error
	for _, ep := range s.endpoints() {
		results, err := s.searchEndpoint(ctx, ep, query)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(results) > 0 {
			return results, nil
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return []SearchSource{}, nil
}

func (s *HTTPSearchClient) endpoints() []string {
	var eps []string
	if s.Endpoint != "" {
		eps = append(eps, s.Endpoint)
	}
	if s.FallbackEndpoint != "" && s.FallbackEndpoint != s.Endpoint {
		eps = append(eps, s.FallbackEndpoint)
	}
	return eps
}

func isWikipedia(endpoint string) bool {
	return strings.Contains(strings.ToLower(endpoint), "wikipedia.org")
}

func buildSearchURL(endpoint, query string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if isWikipedia(endpoint) {
		q.Set("action", "query")
		q.Set("list", "search")
		q.Set("srsearch", query)
		q.Set("srlimit", fmt.Sprint(maxSources))
		q.Set("format", "json")
		q.Set("utf8", "1")
	} else {
		q.Set("q", query)
		q.Set("query", query)
		q.Set("format", "json")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *HTTPSearchClient) searchEndpoint(ctx context.Context, endpoint, query string) ([]SearchSource, error) {
	target, err := buildSearchURL(endpoint, query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if s.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search engine returned status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return ParseSearchResults(body, endpoint), nil
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string) string {
	s = html.UnescapeString(htmlTag.ReplaceAllString(s, ""))
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func findItems(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case map[string]any:
		for _, key := range []string{"results", "data", "items", "organic_results", "organic", "RelatedTopics", "query", "search", "web"} {
			if items := findItems(v[key]); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func wikiURL(endpoint, title string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("https://%s/wiki/%s", u.Host, url.PathEscape(strings.ReplaceAll(title, " ", "_")))
}

// ParseSearchResults mengurai berbagai format respons mesin pencari menjadi daftar SearchSource.
func ParseSearchResults(body []byte, endpoint string) []SearchSource {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return []SearchSource{}
	}

	out := []SearchSource{}
	seen := map[string]bool{}
	for _, item := range findItems(raw) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		title := cleanText(pickString(m, "title", "name", "Text"))
		link := strings.TrimSpace(pickString(m, "url", "link", "href", "FirstURL"))
		snippet := cleanText(pickString(m, "snippet", "content", "description", "body", "text", "Text"))
		if link == "" && title != "" && isWikipedia(endpoint) {
			link = wikiURL(endpoint, title)
		}
		if title == "" || link == "" || seen[link] {
			continue
		}
		seen[link] = true
		out = append(out, SearchSource{Title: truncate(title, 160), URL: link, Snippet: truncate(snippet, maxSnippetLen)})
		if len(out) == maxSources {
			break
		}
	}
	return out
}
