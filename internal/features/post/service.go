package post

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	"app/internal/database"
)

// Error domain artikel.
var (
	ErrPostNotFound    = errors.New("post not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrTitleRequired   = errors.New("title is required")
	ErrContentRequired = errors.New("content is required")
)

// ExcerptLength adalah panjang maksimum ringkasan otomatis (dalam karakter).
const ExcerptLength = 180

// PostService mendefinisikan seluruh use case artikel.
type PostService interface {
	Categories() []string
	ListPublic(filter PostFilter) ([]PostResponse, error)
	GetPublicBySlug(slug string) (*PostResponse, error)
	ListMine(userID uint) ([]PostResponse, error)
	Create(userID uint, req CreatePostRequest) (*PostResponse, error)
	GetMine(userID, id uint) (*PostResponse, error)
	Update(userID, id uint, req UpdatePostRequest) (*PostResponse, error)
	TogglePublish(userID, id uint) (*PostResponse, error)
	Delete(userID, id uint) error
	Stats(userID uint) (*StatsResponse, error)
	AdminListUsers() ([]AdminUserResponse, error)
	AdminListPosts() ([]PostResponse, error)
	AdminListUserPosts(userID uint) ([]PostResponse, error)
}

type postService struct {
	repo PostRepository
}

// NewPostService membuat PostService.
func NewPostService(repo PostRepository) PostService {
	return &postService{repo: repo}
}

// NormalizeCategory menyeragamkan penulisan kategori resmi dan memberi default "Umum".
func NormalizeCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return DefaultCategory
	}
	for _, c := range Categories {
		if strings.EqualFold(c, category) {
			return c
		}
	}
	return category
}

// Slugify mengubah judul menjadi slug URL (huruf kecil, angka, dan tanda hubung).
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := b.String()
	if len(slug) > 180 {
		slug = slug[:180]
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "artikel"
	}
	return slug
}

var (
	mdImage   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdSymbols = regexp.MustCompile("[#*_>`~|]+")
)

// MakeExcerpt membuat ringkasan teks polos dari konten Markdown.
func MakeExcerpt(content string) string {
	text := mdImage.ReplaceAllString(content, "")
	text = mdLink.ReplaceAllString(text, "$1")
	text = mdSymbols.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) <= ExcerptLength {
		return text
	}
	runes := []rune(text)[:ExcerptLength]
	cut := string(runes)
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, ",.;: ") + "…"
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func (s *postService) Categories() []string {
	out := make([]string, len(Categories))
	copy(out, Categories)
	return out
}

func (s *postService) ListPublic(filter PostFilter) ([]PostResponse, error) {
	if c := strings.TrimSpace(filter.Category); strings.EqualFold(c, "semua") || strings.EqualFold(c, "all") {
		filter.Category = ""
	}
	posts, err := s.repo.FindPublished(filter)
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}

func (s *postService) GetPublicBySlug(slug string) (*PostResponse, error) {
	p, err := s.repo.FindPublishedBySlug(strings.TrimSpace(slug))
	if err != nil {
		if isNotFound(err) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *postService) ListMine(userID uint) ([]PostResponse, error) {
	posts, err := s.repo.FindByUser(userID)
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}

func (s *postService) uniqueSlug(title string, excludeID uint) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func setPublished(p *database.Post, published bool) {
	switch {
	case published && !p.IsPublished:
		now := time.Now()
		p.PublishedAt = &now
	case !published:
		p.PublishedAt = nil
	}
	p.IsPublished = published
}

func (s *postService) Create(userID uint, req CreatePostRequest) (*PostResponse, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, ErrTitleRequired
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, ErrContentRequired
	}

	slug, err := s.uniqueSlug(title, 0)
	if err != nil {
		return nil, err
	}

	excerpt := strings.TrimSpace(req.Excerpt)
	if excerpt == "" {
		excerpt = MakeExcerpt(req.Content)
	}

	p := &database.Post{
		UserID:         userID,
		Title:          title,
		Slug:           slug,
		Category:       NormalizeCategory(req.Category),
		Excerpt:        excerpt,
		Content:        req.Content,
		CoverImagePath: strings.TrimSpace(req.CoverImagePath),
	}
	setPublished(p, req.IsPublished)

	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	created, err := s.repo.FindByID(p.ID)
	if err != nil {
		return nil, err
	}
	res := ToPostResponse(created)
	return &res, nil
}

func (s *postService) findOwned(userID, id uint) (*database.Post, error) {
	p, err := s.repo.FindByID(id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	if p.UserID != userID {
		return nil, ErrPostNotFound
	}
	return p, nil
}

func (s *postService) GetMine(userID, id uint) (*PostResponse, error) {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *postService) Update(userID, id uint, req UpdatePostRequest) (*PostResponse, error) {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrTitleRequired
		}
		if title != p.Title {
			slug, err := s.uniqueSlug(title, p.ID)
			if err != nil {
				return nil, err
			}
			p.Slug = slug
		}
		p.Title = title
	}
	if req.Content != nil {
		if strings.TrimSpace(*req.Content) == "" {
			return nil, ErrContentRequired
		}
		p.Content = *req.Content
	}
	if req.Category != nil {
		p.Category = NormalizeCategory(*req.Category)
	}
	if req.Excerpt != nil {
		excerpt := strings.TrimSpace(*req.Excerpt)
		if excerpt == "" {
			excerpt = MakeExcerpt(p.Content)
		}
		p.Excerpt = excerpt
	}
	if req.CoverImagePath != nil {
		p.CoverImagePath = strings.TrimSpace(*req.CoverImagePath)
	}
	if req.IsPublished != nil {
		setPublished(p, *req.IsPublished)
	}

	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *postService) TogglePublish(userID, id uint) (*PostResponse, error) {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return nil, err
	}
	setPublished(p, !p.IsPublished)
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(p)
	return &res, nil
}

func (s *postService) Delete(userID, id uint) error {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(p)
}

func (s *postService) Stats(userID uint) (*StatsResponse, error) {
	total, published, err := s.repo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	stats := newStats(total, published)
	return &stats, nil
}

func (s *postService) AdminListUsers() ([]AdminUserResponse, error) {
	users, err := s.repo.FindUsers()
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.UserStats()
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint]UserStatsRow, len(rows))
	for _, r := range rows {
		byUser[r.UserID] = r
	}

	out := make([]AdminUserResponse, 0, len(users))
	for _, u := range users {
		st := byUser[u.ID]
		out = append(out, AdminUserResponse{
			ID: u.ID, Name: u.Name, Username: u.Username, Email: u.Email, Role: u.Role,
			CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
			TotalPosts: st.Total, TotalPublished: st.Published, TotalDraft: st.Total - st.Published,
		})
	}
	return out, nil
}

func (s *postService) AdminListPosts() ([]PostResponse, error) {
	posts, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}

func (s *postService) AdminListUserPosts(userID uint) ([]PostResponse, error) {
	exists, err := s.repo.UserExists(userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}
	posts, err := s.repo.FindByUser(userID)
	if err != nil {
		return nil, err
	}
	return ToPostResponses(posts), nil
}
