package post

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"app/internal/database"
)

// PostFilter adalah parameter pencarian galeri publik.
type PostFilter struct {
	Query    string
	Category string
}

// PostRepository mengabstraksi akses data artikel dan agregasi untuk admin.
type PostRepository interface {
	FindPublished(filter PostFilter) ([]database.Post, error)
	FindPublishedBySlug(slug string) (*database.Post, error)
	FindByUser(userID uint) ([]database.Post, error)
	FindByID(id uint) (*database.Post, error)
	FindAll() ([]database.Post, error)
	SlugExists(slug string, excludeID uint) (bool, error)
	Create(p *database.Post) error
	Save(p *database.Post) error
	Delete(p *database.Post) error
	CountByUser(userID uint) (total int64, published int64, err error)
	FindUsers() ([]database.User, error)
	UserStats() ([]UserStatsRow, error)
}

type postRepository struct {
	db *gorm.DB
}

// NewPostRepository membuat PostRepository berbasis GORM.
func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{db: db}
}

func (r *postRepository) withAuthor() *gorm.DB {
	return r.db.Preload("User")
}

func (r *postRepository) FindPublished(filter PostFilter) ([]database.Post, error) {
	q := r.withAuthor().Where("is_published = ?", true)
	if term := strings.ToLower(strings.TrimSpace(filter.Query)); term != "" {
		like := "%" + term + "%"
		q = q.Where("(LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ? OR LOWER(category) LIKE ?)", like, like, like, like)
	}
	if cat := strings.TrimSpace(filter.Category); cat != "" {
		q = q.Where("LOWER(category) = LOWER(?)", cat)
	}
	var posts []database.Post
	err := q.Order("published_at DESC").Order("id DESC").Find(&posts).Error
	return posts, err
}

func (r *postRepository) FindPublishedBySlug(slug string) (*database.Post, error) {
	var p database.Post
	if err := r.withAuthor().Where("slug = ? AND is_published = ?", slug, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *postRepository) FindByUser(userID uint) ([]database.Post, error) {
	var posts []database.Post
	err := r.withAuthor().Where("user_id = ?", userID).Order("updated_at DESC").Order("id DESC").Find(&posts).Error
	return posts, err
}

func (r *postRepository) FindByID(id uint) (*database.Post, error) {
	var p database.Post
	if err := r.withAuthor().First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *postRepository) FindAll() ([]database.Post, error) {
	var posts []database.Post
	err := r.withAuthor().Order("created_at DESC").Order("id DESC").Find(&posts).Error
	return posts, err
}

func (r *postRepository) SlugExists(slug string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&database.Post{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *postRepository) Create(p *database.Post) error {
	return r.db.Omit(clause.Associations).Create(p).Error
}

func (r *postRepository) Save(p *database.Post) error {
	return r.db.Omit(clause.Associations).Save(p).Error
}

func (r *postRepository) Delete(p *database.Post) error {
	return r.db.Delete(p).Error
}

func (r *postRepository) CountByUser(userID uint) (int64, int64, error) {
	var total, published int64
	if err := r.db.Model(&database.Post{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	err := r.db.Model(&database.Post{}).Where("user_id = ? AND is_published = ?", userID, true).Count(&published).Error
	return total, published, err
}

func (r *postRepository) FindUsers() ([]database.User, error) {
	var users []database.User
	err := r.db.Order("id ASC").Find(&users).Error
	return users, err
}

func (r *postRepository) UserStats() ([]UserStatsRow, error) {
	var rows []UserStatsRow
	err := r.db.Model(&database.Post{}).
		Select("user_id, COUNT(*) AS total, SUM(CASE WHEN is_published THEN 1 ELSE 0 END) AS published").
		Group("user_id").
		Scan(&rows).Error
	return rows, err
}
