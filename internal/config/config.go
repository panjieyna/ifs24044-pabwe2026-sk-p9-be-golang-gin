// Package config memuat konfigurasi aplikasi dari berkas .env dan environment variable
// dengan penamaan bergaya Laravel (APP_*, DB_*, OPENAI_*, SEARCH_ENGINE_*).
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi runtime aplikasi.
type Config struct {
	AppEnv  string
	AppPort string
	AppKey  string

	AdminUsername string
	AdminPassword string

	DBConnection string
	DBHost       string
	DBPort       string
	DBDatabase   string
	DBUsername   string
	DBPassword   string
	DBSSLMode    string

	SearchEngineEndpoint string
	OpenAIEndpoint       string
	OpenAIKey            string
	OpenAIModel          string
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// LoadConfig membaca berkas .env (opsional, tidak menimpa environment yang sudah ada)
// lalu menyusun Config dengan nilai default yang aman untuk pengembangan lokal.
func LoadConfig(envPath ...string) *Config {
	if len(envPath) > 0 && envPath[0] != "" {
		_ = godotenv.Load(envPath[0])
	} else {
		_ = godotenv.Load()
	}

	return &Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		AppPort: getEnv("APP_PORT", getEnv("PORT", "8081")),
		AppKey:  getEnv("APP_KEY", "delcom_blog_secret_key_super_secure_2026_random_string_32chars"),

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin"),

		DBConnection: getEnv("DB_CONNECTION", "pgsql"),
		DBHost:       getEnv("DB_HOST", "127.0.0.1"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBDatabase:   getEnv("DB_DATABASE", "pabwe_p9_blog"),
		DBUsername:   getEnv("DB_USERNAME", "postgres"),
		DBPassword:   getEnv("DB_PASSWORD", "postgres"),
		DBSSLMode:    getEnv("DB_SSLMODE", "disable"),

		SearchEngineEndpoint: getEnv("SEARCH_ENGINE_ENDPOINT", "https://id.wikipedia.org/w/api.php"),
		OpenAIEndpoint:       getEnv("OPENAI_ENDPOINT", "https://delcom.org/api/v1"),
		OpenAIKey:            getEnv("OPENAI_KEY", ""),
		OpenAIModel:          getEnv("OPENAI_MODEL", "gpt-4o-mini"),
	}
}

// IsProduction bernilai true jika APP_ENV=production.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// DSN menyusun connection string PostgreSQL.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Jakarta connect_timeout=10",
		c.DBHost, c.DBUsername, c.DBPassword, c.DBDatabase, c.DBPort, c.DBSSLMode,
	)
}
