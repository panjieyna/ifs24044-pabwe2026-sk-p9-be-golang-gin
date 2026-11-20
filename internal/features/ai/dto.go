// Package ai menyediakan AI Blog Generator yang terintegrasi dengan mesin pencari:
// topik → penelusuran web → prompt + konteks → draf artikel Markdown beserta sumber rujukan.
package ai

// GeneratePostRequest adalah payload POST /api/posts/ai-generate.
// Salah satu dari topic/prompt/title wajib diisi. include_web_search bawaannya true.
type GeneratePostRequest struct {
	Topic            string `json:"topic" binding:"omitempty,max=500"`
	Prompt           string `json:"prompt" binding:"omitempty,max=500"`
	Title            string `json:"title" binding:"omitempty,max=500"`
	Category         string `json:"category" binding:"omitempty,max=100"`
	Tone             string `json:"tone" binding:"omitempty,max=100"`
	Keywords         string `json:"keywords" binding:"omitempty,max=500"`
	IncludeWebSearch *bool  `json:"include_web_search"`
}

// SearchSource adalah satu hasil penelusuran mesin pencari yang dapat dikutip.
type SearchSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// ArticleInput adalah konteks yang dikirim ke model bahasa.
type ArticleInput struct {
	Topic    string
	Category string
	Tone     string
	Keywords string
	Sources  []SearchSource
}

// ArticleDraft adalah draf artikel yang dihasilkan model.
type ArticleDraft struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Excerpt  string `json:"excerpt"`
	Content  string `json:"content"`
}

// GeneratePostResponse adalah hasil akhir generator yang siap diterapkan ke editor.
type GeneratePostResponse struct {
	Title         string         `json:"title"`
	Category      string         `json:"category"`
	Excerpt       string         `json:"excerpt"`
	Content       string         `json:"content"`
	ReadingTime   int            `json:"reading_time"`
	SearchSources []SearchSource `json:"search_sources"`
	UsedWebSearch bool           `json:"used_web_search"`
	Generator     string         `json:"generator"`
}
