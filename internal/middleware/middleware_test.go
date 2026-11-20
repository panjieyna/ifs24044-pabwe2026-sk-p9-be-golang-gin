package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const secret = "test-secret"

func init() {
	gin.SetMode(gin.TestMode)
}

func sign(t *testing.T, method jwt.SigningMethod, claims *Claims, key string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
	assert.NoError(t, err)
	return token
}

func validClaims(id uint, role string) *Claims {
	return &Claims{
		UserID: id, Username: "user", Role: role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
}

func serve(r *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestCORS(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.OPTIONS("/x", func(c *gin.Context) { c.String(http.StatusOK, "should not reach") })

	w := serve(r, http.MethodOptions, "/x", nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")

	w = serve(r, http.MethodGet, "/x", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestSecurityHeaders(t *testing.T) {
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := serve(r, http.MethodGet, "/x", nil)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestAuth(t *testing.T) {
	r := gin.New()
	r.GET("/me", Auth(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": c.GetUint(CtxUserID), "role": c.GetString(CtxUserRole), "username": c.GetString(CtxUsername)})
	})

	cases := map[string]string{
		"missing header": "",
		"wrong scheme":   "Token abc",
		"empty token":    "Bearer    ",
		"garbage token":  "Bearer not-a-jwt",
		"wrong secret":   "Bearer " + sign(t, jwt.SigningMethodHS256, validClaims(1, "member"), "other"),
		"wrong alg":      "Bearer " + sign(t, jwt.SigningMethodHS512, validClaims(1, "member"), secret),
		"zero user":      "Bearer " + sign(t, jwt.SigningMethodHS256, validClaims(0, "member"), secret),
		"expired": "Bearer " + sign(t, jwt.SigningMethodHS256, &Claims{UserID: 1, RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		}}, secret),
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			w := serve(r, http.MethodGet, "/me", map[string]string{"Authorization": header})
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), `"status":"fail"`)
		})
	}

	w := serve(r, http.MethodGet, "/me", map[string]string{"Authorization": "bearer " + sign(t, jwt.SigningMethodHS256, validClaims(7, "admin"), secret)})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"id":7,"role":"admin","username":"user"}`, w.Body.String())
}

func TestRequireRole(t *testing.T) {
	r := gin.New()
	r.GET("/admin", Auth(secret), RequireRole("admin"), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := serve(r, http.MethodGet, "/admin", map[string]string{"Authorization": "Bearer " + sign(t, jwt.SigningMethodHS256, validClaims(1, "admin"), secret)})
	assert.Equal(t, http.StatusOK, w.Code)

	w = serve(r, http.MethodGet, "/admin", map[string]string{"Authorization": "Bearer " + sign(t, jwt.SigningMethodHS256, validClaims(2, "member"), secret)})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRateLimiter(t *testing.T) {
	r := gin.New()
	r.GET("/x", RateLimiter(2, 50*time.Millisecond), func(c *gin.Context) { c.Status(http.StatusOK) })

	ip := map[string]string{"X-Forwarded-For": "10.1.1.1"}
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", ip).Code)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", ip).Code)
	assert.Equal(t, http.StatusTooManyRequests, serve(r, http.MethodGet, "/x", ip).Code)

	other := map[string]string{"X-Forwarded-For": "10.1.1.2"}
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", other).Code)

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", ip).Code)
}

func TestRateLimiter_EvictsExpiredClients(t *testing.T) {
	r := gin.New()
	r.GET("/x", RateLimiter(1, time.Nanosecond), func(c *gin.Context) { c.Status(http.StatusOK) })
	for i := 0; i <= 10001; i++ {
		ip := map[string]string{"X-Forwarded-For": fmt.Sprintf("10.%d.%d.%d", i/65536%256, i/256%256, i%256)}
		assert.Equal(t, http.StatusOK, serve(r, http.MethodGet, "/x", ip).Code)
	}
}
