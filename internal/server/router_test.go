package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"app/internal/config"
	"app/internal/database"
)

// tempDBPath membuat berkas SQLite sementara. Pembersihan dilakukan best-effort karena
// Windows mengunci berkas selama koneksi milik router masih terbuka.
func tempDBPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "delcom-router-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "blog.db")
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		AppEnv: "testing", AppKey: "router-secret",
		AdminUsername: "admin", AdminPassword: "admin",
		DBConnection: "sqlite", DBDatabase: tempDBPath(t),
		// Mesin pencari & AI sengaja diarahkan ke alamat tidak valid agar tes tidak memanggil jaringan.
		SearchEngineEndpoint: "://offline", OpenAIEndpoint: "", OpenAIKey: "",
	}
}

func newRouter(t *testing.T) *gin.Engine {
	t.Helper()
	r, err := Bootstrap(testConfig(t))
	require.NoError(t, err)
	return r
}

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func do(t *testing.T, r *gin.Engine, method, path, body, token string) (*httptest.ResponseRecorder, envelope) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w, env
}

func login(t *testing.T, r *gin.Engine, username, password string) string {
	t.Helper()
	w, env := do(t, r, http.MethodPost, "/api/auth/login", `{"username":"`+username+`","password":"`+password+`"}`, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var data struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	return data.Token
}

func TestRouter_InfrastructureRoutes(t *testing.T) {
	r := newRouter(t)

	w, env := do(t, r, http.MethodGet, "/random-unmatched-route", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "fail", env.Status)
	assert.Equal(t, "Endpoint not found", env.Message)

	w, _ = do(t, r, http.MethodGet, "/api/nonexistent", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w, env = do(t, r, http.MethodGet, "/api/health", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", env.Status)

	w, _ = do(t, r, http.MethodOptions, "/api/posts", "", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	for _, path := range []string{"/swagger", "/swagger/"} {
		w, _ = do(t, r, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusMovedPermanently, w.Code, path)
		assert.Equal(t, "/swagger/index.html", w.Header().Get("Location"))
	}

	w, _ = do(t, r, http.MethodGet, "/swagger/index.html", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "swagger-ui")

	w, _ = do(t, r, http.MethodGet, "/swagger/swagger-initializer.js", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "/swagger/api.json")

	req := httptest.NewRequest(http.MethodGet, "/swagger/api.json", nil)
	req.Host = "api.myapp.test:8081"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"host": "api.myapp.test:8081"`)
	assert.Contains(t, w.Body.String(), `"http"`)

	req = httptest.NewRequest(http.MethodGet, "/api.json", nil)
	req.Header.Set("X-Forwarded-Host", "blog.delcom.org")
	req.Header.Set("X-Forwarded-Proto", "https")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), `"host": "blog.delcom.org"`)
	assert.Contains(t, w.Body.String(), `"https"`)

	req = httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	req.TLS = &tls.ConnectionState{}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), `"https"`)
}

func TestRouter_EndToEndBlogFlow(t *testing.T) {
	r := newRouter(t)

	w, env := do(t, r, http.MethodGet, "/api/public/posts", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	var seeded []map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &seeded))
	assert.Len(t, seeded, 6)

	w, _ = do(t, r, http.MethodGet, "/api/public/categories", "", "")
	assert.Equal(t, http.StatusOK, w.Code)

	w, _ = do(t, r, http.MethodPost, "/api/auth/register", `{"username":"penulis","email":"penulis@example.com","password":"rahasia1"}`, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	token := login(t, r, "penulis", "rahasia1")

	w, env = do(t, r, http.MethodPost, "/api/posts", `{"title":"Halo Delcom","category":"Teknologi","content":"Isi artikel pertama."}`, token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		ID   uint   `json:"id"`
		Slug string `json:"slug"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &created))
	assert.Equal(t, "halo-delcom", created.Slug)

	w, _ = do(t, r, http.MethodGet, "/api/public/posts/halo-delcom", "", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w, _ = do(t, r, http.MethodPatch, "/api/posts/1000/toggle-publish", "", token)
	assert.Equal(t, http.StatusNotFound, w.Code)
	w, _ = do(t, r, http.MethodPatch, "/api/posts/"+jsonNumber(created.ID)+"/toggle-publish", "", token)
	assert.Equal(t, http.StatusOK, w.Code)

	w, _ = do(t, r, http.MethodGet, "/api/public/posts/halo-delcom", "", "")
	assert.Equal(t, http.StatusOK, w.Code)

	w, env = do(t, r, http.MethodPost, "/api/posts/ai-generate", `{"topic":"Golang untuk pemula","category":"Teknologi","include_web_search":false}`, token)
	assert.Equal(t, http.StatusOK, w.Code)
	var gen map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &gen))
	assert.Equal(t, "fallback", gen["generator"])
	assert.NotEmpty(t, gen["content"])

	w, _ = do(t, r, http.MethodGet, "/api/admin/users", "", token)
	assert.Equal(t, http.StatusForbidden, w.Code)

	admin := login(t, r, "admin", "admin")
	w, _ = do(t, r, http.MethodGet, "/api/admin/users", "", admin)
	assert.Equal(t, http.StatusOK, w.Code)
	w, _ = do(t, r, http.MethodGet, "/api/admin/posts", "", admin)
	assert.Equal(t, http.StatusOK, w.Code)

	w, _ = do(t, r, http.MethodGet, "/api/auth/profile", "", token)
	assert.Equal(t, http.StatusOK, w.Code)
	w, _ = do(t, r, http.MethodPost, "/api/auth/logout", "", token)
	assert.Equal(t, http.StatusOK, w.Code)
}

func jsonNumber(id uint) string {
	raw, _ := json.Marshal(id)
	return string(raw)
}

func TestSetupRouter_ProductionMode(t *testing.T) {
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })
	db, err := database.ConnectTest()
	require.NoError(t, err)
	cfg := testConfig(t)
	cfg.AppEnv = "production"
	r := SetupRouter(cfg, db)
	assert.Equal(t, gin.ReleaseMode, gin.Mode())
	w, _ := do(t, r, http.MethodGet, "/api/health", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRun(t *testing.T) {
	orig := listenAndServe
	t.Cleanup(func() { listenAndServe = orig })

	cfg := testConfig(t)
	cfg.AppPort = "invalid:port:x"
	assert.Error(t, Run(cfg), "real listener must reject an invalid address")

	var addr string
	listenAndServe = func(srv *http.Server) error {
		addr = srv.Addr
		return http.ErrServerClosed
	}
	cfg = testConfig(t)
	cfg.AppPort = "9999"
	assert.NoError(t, Run(cfg))
	assert.Equal(t, ":9999", addr)

	bad := testConfig(t)
	bad.DBConnection, bad.DBHost, bad.DBPort = "pgsql", "127.0.0.1", "1"
	assert.Error(t, Run(bad))
}

func TestBootstrap_Errors(t *testing.T) {
	cfg := testConfig(t)
	cfg.DBConnection = "pgsql"
	cfg.DBHost, cfg.DBPort = "127.0.0.1", "1"
	_, err := Bootstrap(cfg)
	assert.Error(t, err)

	boom := errors.New("boom")
	origMigrate, origSeed := migrateDB, seedDB
	t.Cleanup(func() { migrateDB, seedDB = origMigrate, origSeed })

	migrateDB = func(*gorm.DB) error { return boom }
	_, err = Bootstrap(testConfig(t))
	assert.ErrorContains(t, err, "failed to migrate database")

	migrateDB = origMigrate
	seedDB = func(*gorm.DB, *config.Config) error { return boom }
	_, err = Bootstrap(testConfig(t))
	assert.ErrorContains(t, err, "failed to seed database")
}
