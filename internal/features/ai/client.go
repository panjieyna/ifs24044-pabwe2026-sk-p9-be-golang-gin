package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrAIUnavailable dikembalikan bila OPENAI_ENDPOINT / OPENAI_KEY belum dikonfigurasi.
var ErrAIUnavailable = errors.New("AI service is not configured")

// ChatClient menghasilkan draf artikel dari sebuah model bahasa.
type ChatClient interface {
	GenerateArticle(ctx context.Context, in ArticleInput) (*ArticleDraft, error)
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// OpenAIClient memanggil endpoint Chat Completions yang kompatibel dengan OpenAI.
type OpenAIClient struct {
	Endpoint   string
	Key        string
	Model      string
	HTTPClient *http.Client
}

// NewOpenAIClient membuat OpenAIClient (timeout 60 detik).
func NewOpenAIClient(endpoint, key, model string) *OpenAIClient {
	return &OpenAIClient{
		Endpoint:   strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		Key:        strings.TrimSpace(key),
		Model:      strings.TrimSpace(model),
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

const systemPrompt = `Kamu adalah editor sekaligus penulis blog profesional berbahasa Indonesia.
Tulis artikel orisinal, akurat, dan enak dibaca berdasarkan topik dari pengguna.
Balas HANYA dengan satu objek JSON valid, tanpa blok kode dan tanpa teks lain, dengan format:
{"title": "...", "category": "...", "excerpt": "...", "content": "..."}
Aturan:
- title: judul yang spesifik dan menarik, maksimal 90 karakter.
- category: salah satu dari Teknologi, Pendidikan, Gaya Hidup, Bisnis, Opini, Umum.
- excerpt: 1-2 kalimat pemikat, maksimal 200 karakter.
- content: artikel Markdown 500-900 kata berisi paragraf pembuka, beberapa subjudul "##", dan penutup. Jangan menulis ulang judul sebagai heading.
- Bila diberikan hasil penelusuran mesin pencari, gunakan faktanya, beri sitasi bernomor seperti [1] pada kalimat terkait, dan akhiri content dengan bagian "## Referensi" berisi daftar bernomor tautan sumber.
- Jangan mengarang fakta, angka, atau kutipan yang tidak didukung sumber.`

// BuildUserPrompt menyusun prompt pengguna beserta konteks hasil penelusuran.
func BuildUserPrompt(in ArticleInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topik: %s\n", in.Topic)
	if in.Category != "" {
		fmt.Fprintf(&b, "Kategori: %s\n", in.Category)
	} else {
		b.WriteString("Kategori: pilih yang paling sesuai\n")
	}
	tone := in.Tone
	if tone == "" {
		tone = "informatif dan bersahabat"
	}
	fmt.Fprintf(&b, "Gaya bahasa: %s\n", tone)
	if in.Keywords != "" {
		fmt.Fprintf(&b, "Kata kunci: %s\n", in.Keywords)
	}
	if len(in.Sources) > 0 {
		b.WriteString("\nHasil penelusuran mesin pencari:\n")
		for i, src := range in.Sources {
			fmt.Fprintf(&b, "[%d] %s — %s (%s)\n", i+1, src.Title, src.Snippet, src.URL)
		}
	}
	return b.String()
}

// GenerateArticle mengirim prompt ke model dan mengurai jawaban JSON menjadi ArticleDraft.
func (c *OpenAIClient) GenerateArticle(ctx context.Context, in ArticleInput) (*ArticleDraft, error) {
	if c.Key == "" || c.Endpoint == "" {
		return nil, ErrAIUnavailable
	}

	payload, _ := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: BuildUserPrompt(in)},
		},
		Temperature: 0.7,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI provider returned status %d", resp.StatusCode)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("invalid AI response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("AI response has no choices")
	}
	return ParseArticleDraft(parsed.Choices[0].Message.Content)
}

// ParseArticleDraft mengambil objek JSON dari jawaban model (termasuk bila dibungkus blok kode).
func ParseArticleDraft(content string) (*ArticleDraft, error) {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, errors.New("AI response does not contain a JSON object")
	}

	var draft ArticleDraft
	if err := json.Unmarshal([]byte(content[start:end+1]), &draft); err != nil {
		return nil, fmt.Errorf("AI response is not valid JSON: %w", err)
	}
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Content = strings.TrimSpace(draft.Content)
	draft.Excerpt = strings.TrimSpace(draft.Excerpt)
	if draft.Title == "" || draft.Content == "" {
		return nil, errors.New("AI response is missing title or content")
	}
	return &draft, nil
}
