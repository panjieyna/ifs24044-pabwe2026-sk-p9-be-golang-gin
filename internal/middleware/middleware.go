// Package middleware berisi CORS, header keamanan, autentikasi JWT, pemeriksaan role, dan rate limiter.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"app/internal/response"
)

// Context keys yang diisi oleh middleware Auth.
const (
	CtxUserID   = "userID"
	CtxUsername = "username"
	CtxUserRole = "userRole"
)

// Claims adalah payload token JWT aplikasi.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// CORS mengizinkan frontend yang dideploy terpisah untuk mengakses API (autentikasi via Bearer token).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept, Accept-Encoding, Authorization, Origin, Cache-Control, X-Requested-With")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders menambahkan header keamanan dasar pada setiap respons.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// Auth memverifikasi header "Authorization: Bearer <token>" dan menyimpan identitas pengguna ke context.
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			response.AbortFail(c, http.StatusUnauthorized, "Unauthorized: missing Authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
			response.AbortFail(c, http.StatusUnauthorized, "Unauthorized: invalid Authorization header format")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(strings.TrimSpace(parts[1]), claims, func(*jwt.Token) (any, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid || claims.UserID == 0 {
			response.AbortFail(c, http.StatusUnauthorized, "Unauthorized: invalid or expired token")
			return
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxUserRole, claims.Role)
		c.Next()
	}
}

// RequireRole hanya meneruskan request bila role pengguna termasuk dalam allowedRoles.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(CtxUserRole)
		for _, r := range allowedRoles {
			if r == role {
				c.Next()
				return
			}
		}
		response.AbortFail(c, http.StatusForbidden, "Forbidden: insufficient privileges")
	}
}

type window struct {
	count   int
	resetAt time.Time
}

// RateLimiter membatasi jumlah request per IP dalam jendela waktu tertentu (fixed window).
func RateLimiter(maxRequests int, per time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	clients := make(map[string]*window)

	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()

		mu.Lock()
		if len(clients) > 10000 {
			for k, w := range clients {
				if now.After(w.resetAt) {
					delete(clients, k)
				}
			}
		}
		w, ok := clients[ip]
		if !ok || now.After(w.resetAt) {
			w = &window{resetAt: now.Add(per)}
			clients[ip] = w
		}
		w.count++
		allowed := w.count <= maxRequests
		mu.Unlock()

		if !allowed {
			response.AbortFail(c, http.StatusTooManyRequests, "Too many requests, please slow down")
			return
		}
		c.Next()
	}
}
