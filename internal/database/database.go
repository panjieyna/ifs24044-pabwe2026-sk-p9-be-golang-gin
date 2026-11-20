// Package database mengelola model GORM, koneksi (PostgreSQL untuk runtime, SQLite in-memory
// untuk pengujian), migrasi skema, dan seeder data awal.
package database

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"app/internal/config"
)

// WordsPerMinute adalah kecepatan baca rata-rata untuk estimasi reading time.
const WordsPerMinute = 200

// User adalah akun penulis (member) maupun admin.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255" json:"name"`
	Username  string    `gorm:"size:255;uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:text;not null" json:"-"`
	Role      string    `gorm:"size:50;not null;default:'member'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Post adalah artikel blog dalam format Markdown.
type Post struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	Title          string     `gorm:"size:255;not null" json:"title"`
	Slug           string     `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Category       string     `gorm:"size:100;not null;index" json:"category"`
	Excerpt        string     `gorm:"type:text" json:"excerpt"`
	Content        string     `gorm:"type:text" json:"content"`
	CoverImagePath string     `gorm:"type:text" json:"cover_image_path"`
	IsPublished    bool       `gorm:"not null;default:false;index" json:"is_published"`
	PublishedAt    *time.Time `gorm:"index" json:"published_at"`
	ReadingTime    int        `gorm:"not null;default:1" json:"reading_time"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	User           *User      `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
}

// BeforeSave memastikan estimasi waktu baca selalu sinkron dengan isi artikel.
func (p *Post) BeforeSave(_ *gorm.DB) error {
	p.ReadingTime = CalculateReadingTime(p.Content)
	return nil
}

// CalculateReadingTime menghitung estimasi menit baca (kata / 200 wpm, dibulatkan ke atas, minimal 1).
func CalculateReadingTime(content string) int {
	words := len(strings.Fields(content))
	minutes := int(math.Ceil(float64(words) / WordsPerMinute))
	if minutes < 1 {
		return 1
	}
	return minutes
}

func gormConfig() *gorm.Config {
	return &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
}

// Connect membuka koneksi database sesuai DB_CONNECTION (pgsql | sqlite).
func Connect(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch strings.ToLower(cfg.DBConnection) {
	case "sqlite", "sqlite3":
		dialector = sqlite.Open(cfg.DBDatabase)
	default:
		// Simple protocol: tanpa prepared statement tersimpan, sehingga aman
		// di belakang PgBouncer (transaction pooling) seperti pada Delcom.
		dialector = postgres.New(postgres.Config{DSN: cfg.DSN(), PreferSimpleProtocol: true})
	}

	return open(dialector)
}

func open(dialector gorm.Dialector) (*gorm.DB, error) {
	db, err := gorm.Open(dialector, gormConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return db, nil
}

// ConnectTest membuka SQLite in-memory yang terisolasi (satu koneksi) dan langsung memigrasikan skema.
func ConnectTest() (*gorm.DB, error) {
	db, err := open(sqlite.Open(":memory:"))
	if err == nil {
		sqlDB, _ := db.DB()
		sqlDB.SetMaxOpenConns(1)
		err = Migrate(db)
	}
	return db, err
}

// Migrate menjalankan AutoMigrate untuk seluruh model.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Post{})
}

// HashPassword membuat hash bcrypt dari password.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func ensureUser(db *gorm.DB, name, username, email, password, role string) (*User, error) {
	var user User
	err := db.Where("username = ?", username).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user = User{Name: name, Username: username, Email: email, Password: hashed, Role: role}
	if err := db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Seed membuat akun admin (dari ADMIN_USERNAME/ADMIN_PASSWORD), akun member/member,
// dan beberapa artikel contoh bila tabel posts masih kosong. Aman dipanggil berulang kali.
func Seed(db *gorm.DB, cfg *config.Config) error {
	admin, err := ensureUser(db, "Administrator", cfg.AdminUsername, cfg.AdminUsername+"@delcom.org", cfg.AdminPassword, "admin")
	if err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	member, err := ensureUser(db, "Member Delcom", "member", "member@delcom.org", "member", "member")
	if err != nil {
		return fmt.Errorf("seed member: %w", err)
	}

	var count int64
	if err := db.Model(&Post{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count posts: %w", err)
	}
	if count > 0 {
		return nil
	}

	posts := samplePosts(admin.ID, member.ID, time.Now())
	if err := db.Create(&posts).Error; err != nil {
		return fmt.Errorf("seed posts: %w", err)
	}
	return nil
}
