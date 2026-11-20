package ai

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode"

	"app/internal/database"
	"app/internal/features/post"
)

// ErrTopicRequired dikembalikan bila topic/prompt/title kosong.
var ErrTopicRequired = errors.New("topic is required")

// Nilai field "generator" pada respons.
const (
	GeneratorAI       = "openai"
	GeneratorFallback = "fallback"
)

// AiService mendefinisikan use case AI Blog Generator.
type AiService interface {
	GeneratePost(ctx context.Context, req GeneratePostRequest) (*GeneratePostResponse, error)
}

type aiService struct {
	chat   ChatClient
	search SearchClient
}

// NewAiService membuat AiService.
func NewAiService(chat ChatClient, search SearchClient) AiService {
	return &aiService{chat: chat, search: search}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// GeneratePost menelusuri sumber web (bila diminta), meminta model menulis draf, lalu
// menormalkan hasilnya. Bila layanan AI gagal, draf kerangka berbasis sumber dibuat sebagai cadangan.
func (s *aiService) GeneratePost(ctx context.Context, req GeneratePostRequest) (*GeneratePostResponse, error) {
	topic := firstNonEmpty(req.Topic, req.Prompt, req.Title)
	if topic == "" {
		return nil, ErrTopicRequired
	}

	category := ""
	if strings.TrimSpace(req.Category) != "" {
		category = post.NormalizeCategory(req.Category)
	}

	useSearch := req.IncludeWebSearch == nil || *req.IncludeWebSearch
	sources := []SearchSource{}
	if useSearch {
		found, err := s.search.Search(ctx, topic)
		if err != nil {
			log.Printf("ai: web search failed: %v", err)
		} else if found != nil {
			sources = found
		}
	}

	in := ArticleInput{
		Topic:    topic,
		Category: category,
		Tone:     strings.TrimSpace(req.Tone),
		Keywords: strings.TrimSpace(req.Keywords),
		Sources:  sources,
	}

	generator := GeneratorAI
	draft, err := s.chat.GenerateArticle(ctx, in)
	if err != nil {
		log.Printf("ai: generation failed, using outline fallback: %v", err)
		draft = FallbackDraft(in)
		generator = GeneratorFallback
	}

	if category == "" {
		category = post.NormalizeCategory(draft.Category)
	}
	excerpt := draft.Excerpt
	if excerpt == "" {
		excerpt = post.MakeExcerpt(draft.Content)
	}
	content := EnsureReferences(draft.Content, sources)

	return &GeneratePostResponse{
		Title:         draft.Title,
		Category:      category,
		Excerpt:       excerpt,
		Content:       content,
		ReadingTime:   database.CalculateReadingTime(content),
		SearchSources: sources,
		UsedWebSearch: len(sources) > 0,
		Generator:     generator,
	}, nil
}

// EnsureReferences menambahkan bagian "## Referensi" bila ada sumber dan model belum menuliskannya.
func EnsureReferences(content string, sources []SearchSource) string {
	if len(sources) == 0 || strings.Contains(strings.ToLower(content), "## referensi") {
		return content
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteString("\n\n## Referensi\n\n")
	for i, src := range sources {
		fmt.Fprintf(&b, "%d. [%s](%s)\n", i+1, src.Title, src.URL)
	}
	return b.String()
}

func capitalize(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// FallbackDraft membuat kerangka artikel yang jujur menyatakan dirinya dibuat tanpa AI.
func FallbackDraft(in ArticleInput) *ArticleDraft {
	topic := capitalize(in.Topic)
	var b strings.Builder
	b.WriteString("> Layanan AI sedang tidak tersedia. Draf ini adalah kerangka awal yang disusun otomatis; lengkapi setiap bagian sebelum diterbitkan.\n\n")
	fmt.Fprintf(&b, "## Pendahuluan\n\nJelaskan mengapa **%s** relevan bagi pembaca saat ini dan apa yang akan mereka pelajari dari tulisan ini.\n\n", topic)
	if len(in.Sources) > 0 {
		b.WriteString("## Temuan dari Penelusuran\n\n")
		for i, src := range in.Sources {
			fmt.Fprintf(&b, "- **%s** — %s [%d]\n", src.Title, src.Snippet, i+1)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Poin Pembahasan\n\n1. Latar belakang dan konteks.\n2. Konsep inti beserta contoh nyata.\n3. Tantangan, risiko, atau kesalahpahaman umum.\n4. Langkah praktis yang bisa langsung diterapkan.\n\n")
	b.WriteString("## Penutup\n\nRingkas gagasan utama dan ajak pembaca mengambil satu tindakan konkret.\n")

	return &ArticleDraft{
		Title:    topic + ": Kerangka Tulisan",
		Category: in.Category,
		Excerpt:  fmt.Sprintf("Kerangka awal tulisan tentang %s yang disusun dari hasil penelusuran. Lengkapi sebelum diterbitkan.", in.Topic),
		Content:  b.String(),
	}
}
