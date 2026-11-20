package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------- Search engine ----------

func TestSameHost(t *testing.T) {
	assert.True(t, sameHost("https://delcom.org/api/v1/search", "https://DELCOM.org/api/v1"))
	assert.False(t, sameHost("https://id.wikipedia.org/w/api.php", "https://delcom.org/api/v1"))
	assert.False(t, sameHost("://bad", "https://delcom.org"))
	assert.False(t, sameHost("https://delcom.org", "://bad"))
	assert.False(t, sameHost("relative/path", "relative/path"))
}

func TestNewSearchClient_ForwardsKeyOnlyToSameHost(t *testing.T) {
	assert.Equal(t, "k", NewSearchClient("https://delcom.org/api/v1/search", "https://delcom.org/api/v1", "k").APIKey)
	c := NewSearchClient(" https://id.wikipedia.org/w/api.php ", "https://delcom.org/api/v1", "k")
	assert.Empty(t, c.APIKey)
	assert.Equal(t, "https://id.wikipedia.org/w/api.php", c.Endpoint)
	assert.Equal(t, []string{"https://id.wikipedia.org/w/api.php"}, c.endpoints())
}

func TestBuildSearchURL(t *testing.T) {
	u, err := buildSearchURL("https://id.wikipedia.org/w/api.php", "golang gin")
	require.NoError(t, err)
	assert.Contains(t, u, "list=search")
	assert.Contains(t, u, "srsearch=golang+gin")

	u, err = buildSearchURL("https://search.example/api?key=1", "golang")
	require.NoError(t, err)
	assert.Contains(t, u, "q=golang")
	assert.Contains(t, u, "query=golang")
	assert.Contains(t, u, "key=1")

	_, err = buildSearchURL("://bad", "x")
	assert.Error(t, err)
}

func TestSearch_PrimaryEndpointWithKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		assert.Equal(t, "gin", r.URL.Query().Get("q"))
		_, _ = w.Write([]byte(`{"data":{"results":[{"title":"Gin <b>Web</b>","url":"https://gin-gonic.com","content":"Fast &amp; light"}]}}`))
	}))
	defer srv.Close()

	c := NewSearchClient(srv.URL, srv.URL, "secret")
	res, err := c.Search(context.Background(), " gin ")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, SearchSource{Title: "Gin Web", URL: "https://gin-gonic.com", Snippet: "Fast & light"}, res[0])
}

func TestSearch_FallsBackWhenPrimaryFails(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primary.Close()
	wiki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "query", r.URL.Query().Get("action"))
		assert.NotEmpty(t, r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte(`{"query":{"search":[{"title":"Bahasa Go","snippet":"<span>Go</span> adalah bahasa"}]}}`))
	}))
	defer wiki.Close()

	c := &HTTPSearchClient{Endpoint: primary.URL, FallbackEndpoint: wiki.URL + "/wikipedia.org/w/api.php", HTTPClient: http.DefaultClient}
	res, err := c.Search(context.Background(), "golang")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "Bahasa Go", res[0].Title)
	assert.Contains(t, res[0].URL, "/wiki/Bahasa_Go")
	assert.Equal(t, "Go adalah bahasa", res[0].Snippet)
}

func TestSearch_EmptyAndErrors(t *testing.T) {
	c := NewSearchClient("", "", "")
	res, err := c.Search(context.Background(), "   ")
	assert.NoError(t, err)
	assert.Empty(t, res)

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer empty.Close()
	c = &HTTPSearchClient{Endpoint: empty.URL, FallbackEndpoint: empty.URL, HTTPClient: http.DefaultClient}
	res, err = c.Search(context.Background(), "x")
	assert.NoError(t, err)
	assert.Empty(t, res)

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	c = &HTTPSearchClient{Endpoint: "://bad", FallbackEndpoint: closed.URL, HTTPClient: http.DefaultClient}
	_, err = c.Search(context.Background(), "x")
	assert.Error(t, err)

	notOK := httptest.NewServer(http.NotFoundHandler())
	defer notOK.Close()
	c = &HTTPSearchClient{Endpoint: notOK.URL, HTTPClient: http.DefaultClient}
	_, err = c.Search(context.Background(), "x")
	assert.ErrorContains(t, err, "status 404")

	//nolint:staticcheck // sengaja memakai context nil untuk menguji kegagalan pembuatan request.
	_, err = c.searchEndpoint(nil, notOK.URL, "x")
	assert.Error(t, err)
}

func TestParseSearchResults(t *testing.T) {
	assert.Empty(t, ParseSearchResults([]byte("not json"), ""))
	assert.Empty(t, ParseSearchResults([]byte(`["golang", ["a"], ["b"], ["c"]]`), ""))
	assert.Empty(t, ParseSearchResults([]byte(`{"query":{"search":[{"title":"A"}]}}`), "://wikipedia.org"))

	ddg := ParseSearchResults([]byte(`{"RelatedTopics":[{"Text":"Go language","FirstURL":"https://duckduckgo.com/Go"},{"Text":"no url"}]}`), "https://api.duckduckgo.com")
	require.Len(t, ddg, 1)
	assert.Equal(t, "Go language", ddg[0].Snippet)

	long := strings.Repeat("s", 400)
	items := `[{"name":"A","link":"https://a.test","description":"` + long + `"},{"title":"dup","url":"https://a.test"}`
	for _, u := range []string{"b", "c", "d", "e", "f"} {
		items += `,{"title":"` + u + `","href":"https://` + u + `.test"}`
	}
	res := ParseSearchResults([]byte(`{"organic_results":`+items+`]}`), "")
	require.Len(t, res, maxSources)
	assert.Equal(t, "A", res[0].Title)
	assert.True(t, strings.HasSuffix(res[0].Snippet, "…"))
	assert.Equal(t, "b", res[1].Title)
}

// ---------- OpenAI client ----------

func chatServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		raw, _ := io.ReadAll(r.Body)
		var req chatRequest
		assert.NoError(t, json.Unmarshal(raw, &req))
		assert.Equal(t, "model-x", req.Model)
		assert.Len(t, req.Messages, 2)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func completion(content string) string {
	raw, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}}})
	return string(raw)
}

func TestOpenAIClient_GenerateArticle(t *testing.T) {
	ctx := context.Background()
	in := ArticleInput{Topic: "Gin"}

	_, err := NewOpenAIClient("https://x", "", "m").GenerateArticle(ctx, in)
	assert.ErrorIs(t, err, ErrAIUnavailable)

	_, err = NewOpenAIClient("://bad", "key", "m").GenerateArticle(ctx, in)
	assert.Error(t, err)

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	_, err = NewOpenAIClient(closed.URL, "key", "m").GenerateArticle(ctx, in)
	assert.ErrorContains(t, err, "AI request failed")

	for _, tc := range []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusUnauthorized, `{"error":"bad key"}`, "status 401"},
		{http.StatusOK, `not json`, "invalid AI response"},
		{http.StatusOK, `{"choices":[]}`, "no choices"},
		{http.StatusOK, completion("maaf, tidak bisa"), "does not contain a JSON object"},
	} {
		srv := chatServer(t, tc.status, tc.body)
		_, err := NewOpenAIClient(srv.URL+"/v1/", "key", "model-x").GenerateArticle(ctx, in)
		assert.ErrorContains(t, err, tc.want)
		srv.Close()
	}

	srv := chatServer(t, http.StatusOK, completion("```json\n{\"title\":\" Judul \",\"category\":\"Teknologi\",\"excerpt\":\"Ringkas\",\"content\":\"## Isi\\n\\nTeks\"}\n```"))
	defer srv.Close()
	draft, err := NewOpenAIClient(srv.URL+"/v1", "key", "model-x").GenerateArticle(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, &ArticleDraft{Title: "Judul", Category: "Teknologi", Excerpt: "Ringkas", Content: "## Isi\n\nTeks"}, draft)
}

func TestParseArticleDraft(t *testing.T) {
	_, err := ParseArticleDraft("}{")
	assert.Error(t, err)
	_, err = ParseArticleDraft(`{"title": }`)
	assert.ErrorContains(t, err, "not valid JSON")
	_, err = ParseArticleDraft(`{"title":"A","content":"  "}`)
	assert.ErrorContains(t, err, "missing title or content")
}

func TestBuildUserPrompt(t *testing.T) {
	p := BuildUserPrompt(ArticleInput{Topic: "AI"})
	assert.Contains(t, p, "pilih yang paling sesuai")
	assert.Contains(t, p, "informatif dan bersahabat")
	assert.NotContains(t, p, "Kata kunci")

	p = BuildUserPrompt(ArticleInput{Topic: "AI", Category: "Opini", Tone: "santai", Keywords: "etika",
		Sources: []SearchSource{{Title: "S", URL: "https://s.test", Snippet: "snip"}}})
	assert.Contains(t, p, "Kategori: Opini")
	assert.Contains(t, p, "Gaya bahasa: santai")
	assert.Contains(t, p, "Kata kunci: etika")
	assert.Contains(t, p, "[1] S — snip (https://s.test)")
}

// ---------- Service ----------

type fakeSearch struct {
	res []SearchSource
	err error
	got string
}

func (f *fakeSearch) Search(_ context.Context, q string) ([]SearchSource, error) {
	f.got = q
	return f.res, f.err
}

type fakeChat struct {
	draft *ArticleDraft
	err   error
	in    ArticleInput
}

func (f *fakeChat) GenerateArticle(_ context.Context, in ArticleInput) (*ArticleDraft, error) {
	f.in = in
	return f.draft, f.err
}

func boolPtr(b bool) *bool { return &b }

func TestGeneratePost(t *testing.T) {
	ctx := context.Background()
	sources := []SearchSource{{Title: "Sumber", URL: "https://s.test", Snippet: "fakta"}}

	_, err := NewAiService(&fakeChat{}, &fakeSearch{}).GeneratePost(ctx, GeneratePostRequest{Topic: "  "})
	assert.ErrorIs(t, err, ErrTopicRequired)

	search := &fakeSearch{res: sources}
	chat := &fakeChat{draft: &ArticleDraft{Title: "Judul", Category: "bisnis", Content: "Isi artikel."}}
	res, err := NewAiService(chat, search).GeneratePost(ctx, GeneratePostRequest{Prompt: "startup lokal", Tone: " santai "})
	require.NoError(t, err)
	assert.Equal(t, "startup lokal", search.got)
	assert.Equal(t, "santai", chat.in.Tone)
	assert.Equal(t, "Bisnis", res.Category)
	assert.Equal(t, "Isi artikel.", res.Excerpt)
	assert.Contains(t, res.Content, "## Referensi\n\n1. [Sumber](https://s.test)")
	assert.True(t, res.UsedWebSearch)
	assert.Equal(t, GeneratorAI, res.Generator)
	assert.Equal(t, 1, res.ReadingTime)

	chat = &fakeChat{draft: &ArticleDraft{Title: "J", Category: "Opini", Excerpt: "E", Content: "Isi\n\n## Referensi\n\n1. x"}}
	res, err = NewAiService(chat, &fakeSearch{res: sources}).GeneratePost(ctx, GeneratePostRequest{Title: "x", Category: "teknologi"})
	require.NoError(t, err)
	assert.Equal(t, "Teknologi", res.Category)
	assert.Equal(t, "E", res.Excerpt)
	assert.Equal(t, 1, strings.Count(res.Content, "## Referensi"))

	search = &fakeSearch{err: errBoom}
	res, err = NewAiService(&fakeChat{err: errBoom}, search).GeneratePost(ctx, GeneratePostRequest{Topic: "keamanan siber"})
	require.NoError(t, err)
	assert.Equal(t, GeneratorFallback, res.Generator)
	assert.Equal(t, "Keamanan siber: Kerangka Tulisan", res.Title)
	assert.Equal(t, "Umum", res.Category)
	assert.False(t, res.UsedWebSearch)
	assert.NotNil(t, res.SearchSources)
	assert.NotContains(t, res.Content, "Temuan dari Penelusuran")

	res, err = NewAiService(&fakeChat{err: errBoom}, &fakeSearch{res: sources}).GeneratePost(ctx, GeneratePostRequest{Topic: "ai", Category: "Pendidikan"})
	require.NoError(t, err)
	assert.Contains(t, res.Content, "## Temuan dari Penelusuran")
	assert.Contains(t, res.Content, "## Referensi")
	assert.Equal(t, "Pendidikan", res.Category)

	search = &fakeSearch{res: nil}
	res, err = NewAiService(&fakeChat{draft: &ArticleDraft{Title: "T", Content: "C"}}, search).GeneratePost(ctx, GeneratePostRequest{Topic: "x"})
	require.NoError(t, err)
	assert.Equal(t, []SearchSource{}, res.SearchSources)

	search = &fakeSearch{res: sources}
	res, err = NewAiService(&fakeChat{draft: &ArticleDraft{Title: "T", Content: "C"}}, search).GeneratePost(ctx, GeneratePostRequest{Topic: "x", IncludeWebSearch: boolPtr(false)})
	require.NoError(t, err)
	assert.Empty(t, search.got)
	assert.False(t, res.UsedWebSearch)
	assert.Equal(t, "C", res.Content)
}

// ---------- Handler ----------

type errService struct{}

func (errService) GeneratePost(context.Context, GeneratePostRequest) (*GeneratePostResponse, error) {
	return nil, errBoom
}

func doGenerate(t *testing.T, svc AiService, body string) (int, map[string]any) {
	t.Helper()
	r := gin.New()
	r.POST("/gen", NewAiHandler(svc).GeneratePost)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/gen", bytes.NewBufferString(body)))
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	return w.Code, env
}

func TestHandler(t *testing.T) {
	ok := NewAiService(&fakeChat{draft: &ArticleDraft{Title: "T", Content: "C"}}, &fakeSearch{})
	code, env := doGenerate(t, ok, `{"topic":"Golang"}`)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Article draft generated successfully", env["message"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "T", data["title"])

	fallback := NewAiService(&fakeChat{err: errBoom}, &fakeSearch{})
	code, env = doGenerate(t, fallback, `{"topic":"Golang"}`)
	assert.Equal(t, http.StatusOK, code)
	assert.Contains(t, env["message"], "outline draft")

	code, _ = doGenerate(t, ok, `{`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, env = doGenerate(t, ok, `{"category":"Teknologi"}`)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "topic is required", env["message"])

	code, env = doGenerate(t, errService{}, `{"topic":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error", env["status"])
}
