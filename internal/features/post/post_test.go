package post

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"app/internal/database"
	"app/internal/middleware"
)

const secret = "post-test-secret"

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
}

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type testEnv struct {
	db                       *gorm.DB
	r                        *gin.Engine
	member, other, admin     string
	memberID, otherID, admID uint
}

func token(t *testing.T, id uint, role string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{
		UserID: id, Role: role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	require.NoError(t, err)
	return tok
}

func newRouter(svc PostService) *gin.Engine {
	h := NewPostHandler(svc)
	r := gin.New()
	pub := r.Group("/api/public")
	pub.GET("/categories", h.GetCategories)
	pub.GET("/posts", h.GetPublicPosts)
	pub.GET("/posts/:slug", h.GetPublicPostBySlug)

	p := r.Group("/api/posts", middleware.Auth(secret))
	p.GET("", h.GetMyPosts)
	p.POST("", h.CreatePost)
	p.GET("/stats", h.GetStats)
	p.GET("/:id", h.GetMyPost)
	p.PUT("/:id", h.UpdatePost)
	p.PATCH("/:id/toggle-publish", h.TogglePublish)
	p.DELETE("/:id", h.DeletePost)

	a := r.Group("/api/admin", middleware.Auth(secret), middleware.RequireRole("admin"))
	a.GET("/users", h.AdminGetUsers)
	a.GET("/posts", h.AdminGetPosts)
	a.GET("/users/:id/posts", h.AdminGetUserPosts)
	return r
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	db, err := database.ConnectTest()
	require.NoError(t, err)
	users := []database.User{
		{Name: "Penulis", Username: "penulis", Email: "penulis@example.com", Password: "x", Role: "member"},
		{Name: "Lain", Username: "lain", Email: "lain@example.com", Password: "x", Role: "member"},
		{Name: "Admin", Username: "admin", Email: "admin@example.com", Password: "x", Role: "admin"},
	}
	require.NoError(t, db.Create(&users).Error)
	return &testEnv{
		db: db, r: newRouter(NewPostService(NewPostRepository(db))),
		member: token(t, users[0].ID, "member"), other: token(t, users[1].ID, "member"), admin: token(t, users[2].ID, "admin"),
		memberID: users[0].ID, otherID: users[1].ID, admID: users[2].ID,
	}
}

func call(t *testing.T, r *gin.Engine, method, path, body, tok string) (int, envelope) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	r.ServeHTTP(w, req)
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), w.Body.String())
	return w.Code, env
}

func asPost(t *testing.T, env envelope) PostResponse {
	t.Helper()
	var p PostResponse
	require.NoError(t, json.Unmarshal(env.Data, &p))
	return p
}

func asPosts(t *testing.T, env envelope) []PostResponse {
	t.Helper()
	var ps []PostResponse
	require.NoError(t, json.Unmarshal(env.Data, &ps))
	return ps
}

func create(t *testing.T, e *testEnv, tok, body string) PostResponse {
	t.Helper()
	code, env := call(t, e.r, http.MethodPost, "/api/posts", body, tok)
	require.Equal(t, http.StatusCreated, code, env.Message)
	return asPost(t, env)
}

// ---------- Helper murni ----------

func TestSlugify(t *testing.T) {
	assert.Equal(t, "hello-world", Slugify("Hello, World!"))
	assert.Equal(t, "a-b", Slugify("--A  --  B--"))
	assert.Equal(t, "artikel", Slugify("   ***  "))
	long := Slugify(strings.Repeat("ab ", 100))
	assert.LessOrEqual(t, len(long), 180)
	assert.False(t, strings.HasSuffix(long, "-"))
}

func TestNormalizeCategory(t *testing.T) {
	assert.Equal(t, "Umum", NormalizeCategory("  "))
	assert.Equal(t, "Gaya Hidup", NormalizeCategory("gaya hidup"))
	assert.Equal(t, "Kuliner", NormalizeCategory(" Kuliner "))
}

func TestMakeExcerpt(t *testing.T) {
	assert.Equal(t, "Halo dunia tautan", MakeExcerpt("# Halo **dunia** ![img](a.png) [tautan](https://x.y)"))
	long := MakeExcerpt(strings.Repeat("kalimat panjang, ", 40))
	assert.True(t, strings.HasSuffix(long, "…"))
	assert.LessOrEqual(t, len([]rune(long)), ExcerptLength+1)
	noSpace := MakeExcerpt(strings.Repeat("x", 300))
	assert.Equal(t, ExcerptLength+1, len([]rune(noSpace)))
}

// ---------- Alur end-to-end ----------

func TestPublicEndpoints(t *testing.T) {
	e := setup(t)
	code, env := call(t, e.r, http.MethodGet, "/api/public/categories", "", "")
	assert.Equal(t, http.StatusOK, code)
	var cats []string
	require.NoError(t, json.Unmarshal(env.Data, &cats))
	assert.Equal(t, Categories, cats)

	pub := create(t, e, e.member, `{"title":"Belajar Gin","category":"teknologi","content":"Gin adalah framework web Go yang cepat.","is_published":true}`)
	create(t, e, e.member, `{"title":"Resep Rendang","category":"Gaya Hidup","content":"Rendang dimasak perlahan.","is_published":true}`)
	draft := create(t, e, e.member, `{"title":"Draf Rahasia","content":"Belum terbit."}`)

	code, env = call(t, e.r, http.MethodGet, "/api/public/posts", "", "")
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, asPosts(t, env), 2)

	_, env = call(t, e.r, http.MethodGet, "/api/public/posts?q=FRAMEWORK&category=Teknologi", "", "")
	list := asPosts(t, env)
	require.Len(t, list, 1)
	assert.Equal(t, "Belajar Gin", list[0].Title)
	assert.Equal(t, "penulis", list[0].Author.Username)

	_, env = call(t, e.r, http.MethodGet, "/api/public/posts?search=rendang&category=Semua", "", "")
	assert.Len(t, asPosts(t, env), 1)
	_, env = call(t, e.r, http.MethodGet, "/api/public/posts?category=all", "", "")
	assert.Len(t, asPosts(t, env), 2)

	code, env = call(t, e.r, http.MethodGet, "/api/public/posts/"+pub.Slug, "", "")
	assert.Equal(t, http.StatusOK, code)
	detail := asPost(t, env)
	assert.Equal(t, "Teknologi", detail.Category)
	assert.Equal(t, "published", detail.Status)
	assert.Equal(t, 1, detail.ReadingTime)
	assert.Equal(t, "Penulis", detail.User.Name)

	code, _ = call(t, e.r, http.MethodGet, "/api/public/posts/"+draft.Slug, "", "")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodGet, "/api/public/posts/tidak-ada", "", "")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestAuthorCRUD(t *testing.T) {
	e := setup(t)

	p := create(t, e, e.member, `{"title":"Judul Sama","content":"`+strings.Repeat("kata ", 250)+`"}`)
	assert.Equal(t, "judul-sama", p.Slug)
	assert.Equal(t, "Umum", p.Category)
	assert.Equal(t, 2, p.ReadingTime)
	assert.False(t, p.IsPublished)
	assert.Nil(t, p.PublishedAt)
	assert.NotEmpty(t, p.Excerpt)
	assert.Equal(t, "draft", p.Status)

	dup := create(t, e, e.other, `{"title":"Judul Sama","excerpt":"Ringkas","content":"isi","is_published":true}`)
	assert.Equal(t, "judul-sama-2", dup.Slug)
	assert.Equal(t, "Ringkas", dup.Excerpt)
	assert.NotNil(t, dup.PublishedAt)

	for name, body := range map[string]string{
		"invalid json":  `{`,
		"no title":      `{"content":"isi"}`,
		"blank title":   `{"title":"   ","content":"isi"}`,
		"no content":    `{"title":"A"}`,
		"blank content": `{"title":"A","content":"  "}`,
	} {
		code, env := call(t, e.r, http.MethodPost, "/api/posts", body, e.member)
		assert.Equal(t, http.StatusBadRequest, code, name)
		assert.Equal(t, "fail", env.Status, name)
	}

	code, env := call(t, e.r, http.MethodGet, "/api/posts", "", e.member)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, asPosts(t, env), 1)

	path := fmt.Sprintf("/api/posts/%d", p.ID)
	code, env = call(t, e.r, http.MethodGet, path, "", e.member)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, p.ID, asPost(t, env).ID)

	code, _ = call(t, e.r, http.MethodGet, path, "", e.other)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodGet, "/api/posts/99999", "", e.member)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodGet, "/api/posts/abc", "", e.member)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, e.r, http.MethodGet, "/api/posts/0", "", e.member)
	assert.Equal(t, http.StatusBadRequest, code)

	// Update parsial.
	code, env = call(t, e.r, http.MethodPut, path, `{"title":"Judul Baru","category":"opini","excerpt":"","content":"Konten baru","cover_image_path":" /img/a.png ","is_published":true}`, e.member)
	require.Equal(t, http.StatusOK, code, env.Message)
	up := asPost(t, env)
	assert.Equal(t, "judul-baru", up.Slug)
	assert.Equal(t, "Opini", up.Category)
	assert.Equal(t, "Konten baru", up.Excerpt)
	assert.Equal(t, "/img/a.png", up.CoverImagePath)
	assert.True(t, up.IsPublished)
	assert.NotNil(t, up.PublishedAt)
	assert.Equal(t, 1, up.ReadingTime)

	code, env = call(t, e.r, http.MethodPut, path, `{"title":"Judul Baru","excerpt":"Manual","is_published":true}`, e.member)
	assert.Equal(t, http.StatusOK, code)
	same := asPost(t, env)
	assert.Equal(t, "judul-baru", same.Slug)
	assert.Equal(t, "Manual", same.Excerpt)
	assert.Equal(t, up.PublishedAt.Unix(), same.PublishedAt.Unix())

	code, env = call(t, e.r, http.MethodPut, path, `{"is_published":false}`, e.member)
	assert.Equal(t, http.StatusOK, code)
	assert.Nil(t, asPost(t, env).PublishedAt)

	for name, body := range map[string]string{
		"invalid json":  `{`,
		"blank title":   `{"title":" "}`,
		"blank content": `{"content":" "}`,
	} {
		code, _ := call(t, e.r, http.MethodPut, path, body, e.member)
		assert.Equal(t, http.StatusBadRequest, code, name)
	}
	code, _ = call(t, e.r, http.MethodPut, path, `{"title":"Rebut"}`, e.other)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodPut, "/api/posts/x", `{}`, e.member)
	assert.Equal(t, http.StatusBadRequest, code)

	// Toggle publish.
	code, env = call(t, e.r, http.MethodPatch, path+"/toggle-publish", "", e.member)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Post published successfully", env.Message)
	assert.True(t, asPost(t, env).IsPublished)
	code, env = call(t, e.r, http.MethodPatch, path+"/toggle-publish", "", e.member)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Post moved to draft", env.Message)
	code, _ = call(t, e.r, http.MethodPatch, path+"/toggle-publish", "", e.other)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodPatch, "/api/posts/x/toggle-publish", "", e.member)
	assert.Equal(t, http.StatusBadRequest, code)

	// Statistik.
	create(t, e, e.member, `{"title":"Kedua","content":"isi","is_published":true}`)
	code, env = call(t, e.r, http.MethodGet, "/api/posts/stats", "", e.member)
	assert.Equal(t, http.StatusOK, code)
	var st StatsResponse
	require.NoError(t, json.Unmarshal(env.Data, &st))
	assert.Equal(t, StatsResponse{Total: 2, Published: 1, Draft: 1, TotalTerbit: 1, TotalDraf: 1, TotalSelesai: 1, BelumSelesai: 1}, st)

	// Hapus.
	code, _ = call(t, e.r, http.MethodDelete, path, "", e.other)
	assert.Equal(t, http.StatusNotFound, code)
	code, env = call(t, e.r, http.MethodDelete, path, "", e.member)
	assert.Equal(t, http.StatusOK, code)
	assert.Nil(t, env.Data)
	code, _ = call(t, e.r, http.MethodDelete, path, "", e.member)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodDelete, "/api/posts/x", "", e.member)
	assert.Equal(t, http.StatusBadRequest, code)

	code, _ = call(t, e.r, http.MethodGet, "/api/posts", "", "")
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestAdminEndpoints(t *testing.T) {
	e := setup(t)
	create(t, e, e.member, `{"title":"Satu","content":"isi","is_published":true}`)
	create(t, e, e.member, `{"title":"Dua","content":"isi"}`)
	create(t, e, e.other, `{"title":"Tiga","content":"isi"}`)

	code, _ := call(t, e.r, http.MethodGet, "/api/admin/users", "", e.member)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = call(t, e.r, http.MethodGet, "/api/admin/users", "", "")
	assert.Equal(t, http.StatusUnauthorized, code)

	code, env := call(t, e.r, http.MethodGet, "/api/admin/users", "", e.admin)
	assert.Equal(t, http.StatusOK, code)
	var users []AdminUserResponse
	require.NoError(t, json.Unmarshal(env.Data, &users))
	require.Len(t, users, 3)
	assert.Equal(t, "penulis", users[0].Username)
	assert.Equal(t, int64(2), users[0].TotalPosts)
	assert.Equal(t, int64(1), users[0].TotalPublished)
	assert.Equal(t, int64(1), users[0].TotalDraft)
	assert.Equal(t, int64(0), users[2].TotalPosts)

	code, env = call(t, e.r, http.MethodGet, "/api/admin/posts", "", e.admin)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, asPosts(t, env), 3)

	code, env = call(t, e.r, http.MethodGet, fmt.Sprintf("/api/admin/users/%d/posts", e.memberID), "", e.admin)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, asPosts(t, env), 2)

	code, _ = call(t, e.r, http.MethodGet, "/api/admin/users/9999/posts", "", e.admin)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, e.r, http.MethodGet, "/api/admin/users/abc/posts", "", e.admin)
	assert.Equal(t, http.StatusBadRequest, code)
}

// ---------- Jalur error ----------

type faultyRepo struct {
	PostRepository
	failOn string
}

func (f *faultyRepo) err(op string) error {
	if f.failOn == op {
		return errBoom
	}
	return nil
}

func (f *faultyRepo) FindPublished(fl PostFilter) ([]database.Post, error) {
	if e := f.err("FindPublished"); e != nil {
		return nil, e
	}
	return f.PostRepository.FindPublished(fl)
}

func (f *faultyRepo) FindPublishedBySlug(s string) (*database.Post, error) {
	if e := f.err("FindPublishedBySlug"); e != nil {
		return nil, e
	}
	return f.PostRepository.FindPublishedBySlug(s)
}

func (f *faultyRepo) FindByUser(id uint) ([]database.Post, error) {
	if e := f.err("FindByUser"); e != nil {
		return nil, e
	}
	return f.PostRepository.FindByUser(id)
}

func (f *faultyRepo) FindByID(id uint) (*database.Post, error) {
	if e := f.err("FindByID"); e != nil {
		return nil, e
	}
	return f.PostRepository.FindByID(id)
}

func (f *faultyRepo) FindAll() ([]database.Post, error) {
	if e := f.err("FindAll"); e != nil {
		return nil, e
	}
	return f.PostRepository.FindAll()
}

func (f *faultyRepo) SlugExists(s string, id uint) (bool, error) {
	if e := f.err("SlugExists"); e != nil {
		return false, e
	}
	return f.PostRepository.SlugExists(s, id)
}

func (f *faultyRepo) Create(p *database.Post) error {
	if e := f.err("Create"); e != nil {
		return e
	}
	return f.PostRepository.Create(p)
}

func (f *faultyRepo) Save(p *database.Post) error {
	if e := f.err("Save"); e != nil {
		return e
	}
	return f.PostRepository.Save(p)
}

func (f *faultyRepo) CountByUser(id uint) (int64, int64, error) {
	if e := f.err("CountByUser"); e != nil {
		return 0, 0, e
	}
	return f.PostRepository.CountByUser(id)
}

func (f *faultyRepo) FindUsers() ([]database.User, error) {
	if e := f.err("FindUsers"); e != nil {
		return nil, e
	}
	return f.PostRepository.FindUsers()
}

func (f *faultyRepo) UserStats() ([]UserStatsRow, error) {
	if e := f.err("UserStats"); e != nil {
		return nil, e
	}
	return f.PostRepository.UserStats()
}

func (f *faultyRepo) UserExists(id uint) (bool, error) {
	if e := f.err("UserExists"); e != nil {
		return false, e
	}
	return f.PostRepository.UserExists(id)
}

func TestServiceAndHandlerErrors(t *testing.T) {
	e := setup(t)
	existing := create(t, e, e.member, `{"title":"Ada","content":"isi","is_published":true}`)
	path := fmt.Sprintf("/api/posts/%d", existing.ID)

	type req struct{ method, path, body, tok string }
	cases := []struct {
		failOn string
		req    req
		code   int
	}{
		{"FindPublished", req{http.MethodGet, "/api/public/posts", "", ""}, 500},
		{"FindPublishedBySlug", req{http.MethodGet, "/api/public/posts/ada", "", ""}, 500},
		{"FindByUser", req{http.MethodGet, "/api/posts", "", e.member}, 500},
		{"SlugExists", req{http.MethodPost, "/api/posts", `{"title":"B","content":"c"}`, e.member}, 500},
		{"Create", req{http.MethodPost, "/api/posts", `{"title":"B","content":"c"}`, e.member}, 500},
		{"FindByID", req{http.MethodPost, "/api/posts", `{"title":"B","content":"c"}`, e.member}, 500},
		{"FindByID", req{http.MethodGet, path, "", e.member}, 500},
		{"SlugExists", req{http.MethodPut, path, `{"title":"Judul Lain"}`, e.member}, 500},
		{"Save", req{http.MethodPut, path, `{"title":"Judul Lain"}`, e.member}, 500},
		{"Save", req{http.MethodPatch, path + "/toggle-publish", "", e.member}, 500},
		{"CountByUser", req{http.MethodGet, "/api/posts/stats", "", e.member}, 500},
		{"FindUsers", req{http.MethodGet, "/api/admin/users", "", e.admin}, 500},
		{"UserStats", req{http.MethodGet, "/api/admin/users", "", e.admin}, 500},
		{"FindAll", req{http.MethodGet, "/api/admin/posts", "", e.admin}, 500},
		{"UserExists", req{http.MethodGet, fmt.Sprintf("/api/admin/users/%d/posts", e.memberID), "", e.admin}, 500},
		{"FindByUser", req{http.MethodGet, fmt.Sprintf("/api/admin/users/%d/posts", e.memberID), "", e.admin}, 500},
	}
	for _, tc := range cases {
		t.Run(tc.failOn+" "+tc.req.method+" "+tc.req.path, func(t *testing.T) {
			r := newRouter(NewPostService(&faultyRepo{PostRepository: NewPostRepository(e.db), failOn: tc.failOn}))
			code, env := call(t, r, tc.req.method, tc.req.path, tc.req.body, tc.req.tok)
			assert.Equal(t, tc.code, code)
			assert.Equal(t, "error", env.Status)
		})
	}

	// Delete yang gagal di level database.
	svc := NewPostService(NewPostRepository(e.db))
	require.NoError(t, e.db.Callback().Delete().Before("gorm:delete").Register("test:fail_delete", func(tx *gorm.DB) {
		_ = tx.AddError(errBoom)
	}))
	code, _ := call(t, newRouter(svc), http.MethodDelete, path, "", e.member)
	assert.Equal(t, http.StatusInternalServerError, code)
}

func TestRepositoryCountErrors(t *testing.T) {
	db, err := database.ConnectTest()
	require.NoError(t, err)
	repo := NewPostRepository(db)

	calls := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:fail_second", func(tx *gorm.DB) {
		calls++
		if calls == 2 {
			_ = tx.AddError(errBoom)
		}
	}))
	_, _, err = repo.CountByUser(1)
	assert.ErrorIs(t, err, errBoom)

	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())
	_, _, err = repo.CountByUser(1)
	assert.Error(t, err)
}
