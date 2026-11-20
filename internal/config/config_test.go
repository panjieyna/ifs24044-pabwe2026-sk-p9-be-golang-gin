package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var envKeys = []string{
	"APP_ENV", "APP_PORT", "PORT", "APP_KEY", "ADMIN_USERNAME", "ADMIN_PASSWORD",
	"DB_CONNECTION", "DB_HOST", "DB_PORT", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD", "DB_SSLMODE",
	"SEARCH_ENGINE_ENDPOINT", "OPENAI_ENDPOINT", "OPENAI_KEY", "OPENAI_MODEL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	clearEnv(t)
	cfg := LoadConfig(filepath.Join(t.TempDir(), "missing.env"))

	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, "8081", cfg.AppPort)
	assert.Equal(t, "admin", cfg.AdminUsername)
	assert.Equal(t, "admin", cfg.AdminPassword)
	assert.Equal(t, "pgsql", cfg.DBConnection)
	assert.Equal(t, "pabwe_p9_blog", cfg.DBDatabase)
	assert.Equal(t, "https://id.wikipedia.org/w/api.php", cfg.SearchEngineEndpoint)
	assert.Equal(t, "https://delcom.org/api/v1", cfg.OpenAIEndpoint)
	assert.Empty(t, cfg.OpenAIKey)
	assert.NotEmpty(t, cfg.AppKey)
	assert.False(t, cfg.IsProduction())
}

func TestLoadConfig_DefaultDotEnvLookup(t *testing.T) {
	clearEnv(t)
	cfg := LoadConfig()
	assert.Equal(t, "8081", cfg.AppPort)
}

func TestLoadConfig_PortFallback(t *testing.T) {
	clearEnv(t)
	t.Setenv("PORT", "7000")
	assert.Equal(t, "7000", LoadConfig("").AppPort)
}

func TestLoadConfig_FromEnvFile(t *testing.T) {
	clearEnv(t)
	path := filepath.Join(t.TempDir(), ".env")
	content := "APP_ENV=production\nAPP_PORT=9090\nDB_CONNECTION=sqlite\nDB_HOST=db.local\nDB_PORT=6543\n" +
		"DB_DATABASE=blog\nDB_USERNAME=u\nDB_PASSWORD=p\nDB_SSLMODE=require\nOPENAI_KEY=secret\n"
	assert.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg := LoadConfig(path)
	assert.True(t, cfg.IsProduction())
	assert.Equal(t, "9090", cfg.AppPort)
	assert.Equal(t, "sqlite", cfg.DBConnection)
	assert.Equal(t, "secret", cfg.OpenAIKey)
	assert.Equal(t,
		"host=db.local user=u password=p dbname=blog port=6543 sslmode=require TimeZone=Asia/Jakarta connect_timeout=10",
		cfg.DSN())
}
