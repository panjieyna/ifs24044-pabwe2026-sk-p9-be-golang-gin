package auth

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"app/internal/middleware"
	"app/internal/response"
)

// AuthHandler adalah controller endpoint /api/auth/*.
type AuthHandler struct {
	service AuthService
}

// NewAuthHandler membuat AuthHandler.
func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrIdentifierRequired),
		errors.Is(err, ErrUsernameTaken), errors.Is(err, ErrEmailTaken):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		log.Printf("auth: %v", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error")
	}
}

// Register godoc: POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationMessage(err))
		return
	}
	res, err := h.service.Register(req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "User registered successfully", res)
}

// Login godoc: POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationMessage(err))
		return
	}
	res, err := h.service.Login(req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Login successful", res)
}

// Logout godoc: POST /api/auth/logout (token JWT bersifat stateless; klien cukup menghapus token).
func (h *AuthHandler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logged out successfully")
}

// GetProfile godoc: GET /api/auth/profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	res, err := h.service.GetProfile(c.GetUint(middleware.CtxUserID))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profile retrieved successfully", res)
}

// UpdateProfile godoc: PUT /api/auth/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationMessage(err))
		return
	}
	res, err := h.service.UpdateProfile(c.GetUint(middleware.CtxUserID), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profile updated successfully", res)
}
