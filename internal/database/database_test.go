package database

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"app/internal/config"
)

var errBoom = errors.New("boom")

func testCfg() *config.Config {
	return &config.Config{AdminUsername: "admin", AdminPassword: "admin"}
}

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := ConnectTest()
	require.NoError(t, err)
	return db
}

// failOn mendaftarkan callback GORM yang menggagalkan operasi pada tabel tertentu.
func failOn(t *testing.T, db *gorm.DB, op, table string) {
	t.Helper()
	fn := func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			_ = tx.AddError(errBoom)
		}
	}
	var err error
	switch op {
	case "create":
		err = db.Callback().Create().Before("gorm:create").Register("test:fail_"+table, fn)
	case "query":
		err = db.Callback().Query().Before("gorm:query").Register("test:fail_"+table, fn)
	}
	require.NoError(t, err)
}

func TestCalculateReadingTime(t *testing.T) {
	assert.Equal(t, 1, CalculateReadingTime(""))
	assert.Equal(t, 1, CalculateReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, CalculateReadingTime(strings.Repeat("kata ", 201)))
	assert.Equal(t, 3, CalculateReadingTime(strings.Repeat("kata ", 450)))
}

func TestConnect_SqliteFile(t *testing.T) {
	cfg := &config.Config{DBConnection: "sqlite", DBDatabase: filepath.Join(t.TempDir(), "blog.db")}
	db, err := Connect(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	assert.NoError(t, Migrate(db))
	assert.True(t, db.Migrator().HasTable(&Post{}))
}

func TestConnect_PostgresUnreachable(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "pgsql", DBHost: "127.0.0.1", DBPort: "1",
		DBDatabase: "x", DBUsername: "x", DBPassword: "x", DBSSLMode: "disable",
	}
	db, err := Connect(cfg)
	assert.Nil(t, db)
	assert.ErrorContains(t, err, "failed to connect to database")
}

func TestHashPassword(t *testing.T) {
	hashed, err := HashPassword("secret")
	assert.NoError(t, err)
	assert.NotEqual(t, "secret", hashed)

	_, err = HashPassword(strings.Repeat("a", 73))
	assert.Error(t, err)
}

func TestSeed_CreatesAccountsAndPostsIdempotently(t *testing.T) {
	db := newDB(t)
	require.NoError(t, Seed(db, testCfg()))
	require.NoError(t, Seed(db, testCfg()))

	var admin, member User
	require.NoError(t, db.Where("username = ?", "admin").First(&admin).Error)
	require.NoError(t, db.Where("username = ?", "member").First(&member).Error)
	assert.Equal(t, "admin", admin.Role)
	assert.Equal(t, "member", member.Role)

	var users, posts, published int64
	db.Model(&User{}).Count(&users)
	db.Model(&Post{}).Count(&posts)
	db.Model(&Post{}).Where("is_published = ?", true).Count(&published)
	assert.Equal(t, int64(2), users)
	assert.Equal(t, int64(7), posts)
	assert.Equal(t, int64(6), published)

	var p Post
	require.NoError(t, db.Where("slug = ?", "selamat-datang-di-delcom-blog").First(&p).Error)
	assert.Equal(t, admin.ID, p.UserID)
	assert.GreaterOrEqual(t, p.ReadingTime, 1)
	assert.NotNil(t, p.PublishedAt)
}

func TestSeed_AdminLookupFails(t *testing.T) {
	db := newDB(t)
	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())
	assert.ErrorContains(t, Seed(db, testCfg()), "seed admin")
}

func TestSeed_AdminPasswordTooLong(t *testing.T) {
	db := newDB(t)
	cfg := &config.Config{AdminUsername: "admin", AdminPassword: strings.Repeat("x", 80)}
	assert.ErrorContains(t, Seed(db, cfg), "seed admin")
}

func TestSeed_AdminCreateFails(t *testing.T) {
	db := newDB(t)
	failOn(t, db, "create", "users")
	assert.ErrorContains(t, Seed(db, testCfg()), "seed admin")
}

func TestSeed_MemberCreateFails(t *testing.T) {
	db := newDB(t)
	_, err := ensureUser(db, "Administrator", "admin", "admin@delcom.org", "admin", "admin")
	require.NoError(t, err)
	failOn(t, db, "create", "users")
	assert.ErrorContains(t, Seed(db, testCfg()), "seed member")
}

func TestSeed_CountPostsFails(t *testing.T) {
	db := newDB(t)
	failOn(t, db, "query", "posts")
	assert.ErrorContains(t, Seed(db, testCfg()), "count posts")
}

func TestSeed_CreatePostsFails(t *testing.T) {
	db := newDB(t)
	failOn(t, db, "create", "posts")
	assert.ErrorContains(t, Seed(db, testCfg()), "seed posts")
}
