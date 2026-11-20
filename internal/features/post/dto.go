// Package post menangani artikel blog: galeri publik, workspace penulis (CRUD, slug unik,
// reading time, statistik), dan monitoring admin.
package post

import (
	"time"

	"app/internal/database"
)

// Categories adalah daftar kategori resmi Delcom Blog.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// DefaultCategory dipakai bila kategori tidak diisi.
const DefaultCategory = "Umum"

// CreatePostRequest adalah payload pembuatan artikel.
type CreatePostRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Category       string `json:"category" binding:"omitempty,max=100"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
	IsPublished    bool   `json:"is_published"`
}

// UpdatePostRequest memperbarui artikel secara parsial (field nil tidak diubah).
type UpdatePostRequest struct {
	Title          *string `json:"title" binding:"omitempty,max=255"`
	Category       *string `json:"category" binding:"omitempty,max=100"`
	Excerpt        *string `json:"excerpt"`
	Content        *string `json:"content"`
	CoverImagePath *string `json:"cover_image_path"`
	IsPublished    *bool   `json:"is_published"`
}

// AuthorResponse adalah info penulis yang aman ditampilkan publik.
type AuthorResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// PostResponse adalah representasi artikel pada respons API.
type PostResponse struct {
	ID             uint            `json:"id"`
	UserID         uint            `json:"user_id"`
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Category       string          `json:"category"`
	Excerpt        string          `json:"excerpt"`
	Content        string          `json:"content"`
	CoverImagePath string          `json:"cover_image_path"`
	IsPublished    bool            `json:"is_published"`
	Status         string          `json:"status"`
	PublishedAt    *time.Time      `json:"published_at"`
	ReadingTime    int             `json:"reading_time"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Author         *AuthorResponse `json:"author,omitempty"`
	User           *AuthorResponse `json:"user,omitempty"`
}

// StatsResponse adalah ringkasan 3 kartu statistik penulis.
// Alias berbahasa Indonesia disertakan agar kompatibel dengan berbagai klien.
type StatsResponse struct {
	Total        int64 `json:"total"`
	Published    int64 `json:"published"`
	Draft        int64 `json:"draft"`
	TotalTerbit  int64 `json:"total_terbit"`
	TotalDraf    int64 `json:"total_draf"`
	TotalSelesai int64 `json:"total_selesai"`
	BelumSelesai int64 `json:"belum_selesai"`
}

// UserStatsRow adalah hasil agregasi artikel per pengguna.
type UserStatsRow struct {
	UserID    uint
	Total     int64
	Published int64
}

// AdminUserResponse adalah baris direktori pengguna pada panel admin.
type AdminUserResponse struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	TotalDraft     int64     `json:"total_draft"`
}

func newStats(total, published int64) StatsResponse {
	draft := total - published
	return StatsResponse{
		Total: total, Published: published, Draft: draft,
		TotalTerbit: published, TotalDraf: draft,
		TotalSelesai: published, BelumSelesai: draft,
	}
}

// ToPostResponse memetakan model Post (beserta relasi User bila dimuat) menjadi PostResponse.
func ToPostResponse(p *database.Post) PostResponse {
	status := "draft"
	if p.IsPublished {
		status = "published"
	}
	res := PostResponse{
		ID: p.ID, UserID: p.UserID, Title: p.Title, Slug: p.Slug, Category: p.Category,
		Excerpt: p.Excerpt, Content: p.Content, CoverImagePath: p.CoverImagePath,
		IsPublished: p.IsPublished, Status: status, PublishedAt: p.PublishedAt,
		ReadingTime: p.ReadingTime, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if p.User != nil {
		author := &AuthorResponse{ID: p.User.ID, Name: p.User.Name, Username: p.User.Username}
		res.Author = author
		res.User = author
	}
	return res
}

// ToPostResponses memetakan slice Post menjadi slice PostResponse (tidak pernah nil).
func ToPostResponses(posts []database.Post) []PostResponse {
	out := make([]PostResponse, 0, len(posts))
	for i := range posts {
		out = append(out, ToPostResponse(&posts[i]))
	}
	return out
}
