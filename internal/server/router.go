// Package server merangkai seluruh dependensi, rute API publik/penulis/admin, Swagger UI,
// dan handler 404 menjadi sebuah *gin.Engine.
package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	"app/docs"
	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"
)

// Dapat diganti saat pengujian.
var (
	migrateDB      = database.Migrate
	seedDB         = database.Seed
	listenAndServe = func(srv *http.Server) error { return srv.ListenAndServe() }
)

// Run menyiapkan aplikasi lalu melayani HTTP pada APP_PORT hingga server berhenti.
func Run(cfg *config.Config) error {
	router, err := Bootstrap(cfg)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("Delcom Blog API listening on http://127.0.0.1:%s (Swagger UI: /swagger/index.html)", cfg.AppPort)
	if err := listenAndServe(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Bootstrap menghubungkan database, menjalankan migrasi & seeder, lalu menyiapkan router.
func Bootstrap(cfg *config.Config) (*gin.Engine, error) {
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, err
	}
	if err := migrateDB(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}
	if err := seedDB(db, cfg); err != nil {
		return nil, fmt.Errorf("failed to seed database: %w", err)
	}
	return SetupRouter(cfg, db), nil
}

func swaggerJSON(c *gin.Context) {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", docs.GetSwaggerJSON(host, scheme))
}

// SetupRouter mendaftarkan seluruh rute aplikasi.
func SetupRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if cfg.AppEnv != "testing" {
		r.Use(gin.Logger())
	}
	r.Use(gin.Recovery(), middleware.CORS(), middleware.SecurityHeaders())

	// Dependency wiring (feature-first).
	authHdr := auth.NewAuthHandler(auth.NewAuthService(auth.NewUserRepository(db), cfg.AppKey))
	postHdr := post.NewPostHandler(post.NewPostService(post.NewPostRepository(db)))
	aiHdr := ai.NewAiHandler(ai.NewAiService(
		ai.NewOpenAIClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel),
		ai.NewSearchClient(cfg.SearchEngineEndpoint, cfg.OpenAIEndpoint, cfg.OpenAIKey),
	))

	requireAuth := middleware.Auth(cfg.AppKey)
	authLimiter := middleware.RateLimiter(120, time.Minute)
	aiLimiter := middleware.RateLimiter(30, time.Minute)

	// Swagger UI & spesifikasi OpenAPI dinamis.
	swaggerUI := ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/api.json"))
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", func(c *gin.Context) {
		switch c.Param("any") {
		case "", "/":
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
		case "/api.json", "/doc.json":
			swaggerJSON(c)
		default:
			swaggerUI(c)
		}
	})
	r.GET("/api.json", swaggerJSON)

	api := r.Group("/api")

	api.GET("/health", func(c *gin.Context) {
		response.Success(c, http.StatusOK, "Delcom Blog API is running", gin.H{
			"name": "Delcom Blog API", "version": "1.0.0", "time": time.Now().UTC(),
		})
	})

	// Tier 1 — Publik (tanpa login).
	public := api.Group("/public")
	public.GET("/categories", postHdr.GetCategories)
	public.GET("/posts", postHdr.GetPublicPosts)
	public.GET("/posts/:slug", postHdr.GetPublicPostBySlug)

	// Autentikasi.
	authRoutes := api.Group("/auth")
	authRoutes.POST("/register", authLimiter, authHdr.Register)
	authRoutes.POST("/login", authLimiter, authHdr.Login)
	authRoutes.POST("/logout", authHdr.Logout)
	authRoutes.GET("/profile", requireAuth, authHdr.GetProfile)
	authRoutes.PUT("/profile", requireAuth, authHdr.UpdateProfile)

	// Tier 2 — Penulis (perlu login).
	posts := api.Group("/posts", requireAuth)
	posts.GET("", postHdr.GetMyPosts)
	posts.POST("", postHdr.CreatePost)
	posts.GET("/stats", postHdr.GetStats)
	posts.POST("/ai-generate", aiLimiter, aiHdr.GeneratePost)
	posts.GET("/:id", postHdr.GetMyPost)
	posts.PUT("/:id", postHdr.UpdatePost)
	posts.PATCH("/:id/toggle-publish", postHdr.TogglePublish)
	posts.DELETE("/:id", postHdr.DeletePost)

	// Tier 3 — Admin.
	admin := api.Group("/admin", requireAuth, middleware.RequireRole("admin"))
	admin.GET("/users", postHdr.AdminGetUsers)
	admin.GET("/posts", postHdr.AdminGetPosts)
	admin.GET("/users/:id/posts", postHdr.AdminGetUserPosts)

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})

	return r
}
