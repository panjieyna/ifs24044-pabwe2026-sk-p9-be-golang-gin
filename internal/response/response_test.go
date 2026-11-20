package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func run(handler gin.HandlerFunc) (int, map[string]any) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	handler(c)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestSuccess(t *testing.T) {
	code, body := run(func(c *gin.Context) { Success(c, http.StatusCreated, "created", gin.H{"id": 1}) })
	assert.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "created", body["message"])
	assert.NotNil(t, body["data"])

	_, body = run(func(c *gin.Context) { Success(c, http.StatusOK, "ok") })
	_, hasData := body["data"]
	assert.False(t, hasData)

	_, body = run(func(c *gin.Context) { Success(c, http.StatusOK, "ok", nil) })
	_, hasData = body["data"]
	assert.False(t, hasData)
}

func TestFailAndError(t *testing.T) {
	code, body := run(func(c *gin.Context) { Fail(c, http.StatusBadRequest, "bad") })
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "fail", body["status"])

	code, body = run(func(c *gin.Context) { Error(c, http.StatusInternalServerError, "boom") })
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error", body["status"])

	code, body = run(func(c *gin.Context) {
		AbortFail(c, http.StatusUnauthorized, "nope")
		assert.True(t, c.IsAborted())
	})
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "fail", body["status"])

	code, body = run(func(c *gin.Context) {
		AbortError(c, http.StatusServiceUnavailable, "down")
		assert.True(t, c.IsAborted())
	})
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "error", body["status"])
}

type sample struct {
	UserName string `validate:"required"`
	Email    string `validate:"email"`
	Password string `validate:"min=6"`
	Title    string `validate:"max=3"`
	Role     string `validate:"oneof=admin member"`
}

func TestValidationMessage(t *testing.T) {
	err := validator.New().Struct(sample{Email: "x", Password: "1", Title: "long", Role: "x"})
	msg := ValidationMessage(err)
	assert.Contains(t, msg, "Validation failed:")
	assert.Contains(t, msg, "user_name is required")
	assert.Contains(t, msg, "email must be a valid email address")
	assert.Contains(t, msg, "password must be at least 6 characters")
	assert.Contains(t, msg, "title must be at most 3 characters")
	assert.Contains(t, msg, "role is invalid")

	assert.Equal(t, "Invalid request body", ValidationMessage(errors.New("unexpected EOF")))
}
