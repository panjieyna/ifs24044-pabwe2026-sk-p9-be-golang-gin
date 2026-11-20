// Package auth menangani registrasi penulis, login JWT, logout, dan profil akun.
package auth

import (
	"time"

	"app/internal/database"
)

// RegisterRequest adalah payload pendaftaran akun baru.
// Role opsional: hanya "admin" atau "member" yang diterima, selain itu menjadi "member".
type RegisterRequest struct {
	Name     string `json:"name" binding:"omitempty,max=255"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=6,max=72"`
	Role     string `json:"role"`
}

// LoginRequest menerima username atau email beserta password.
type LoginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
}

// UpdateProfileRequest memperbarui data profil secara parsial.
type UpdateProfileRequest struct {
	Name     *string `json:"name" binding:"omitempty,max=255"`
	Email    string  `json:"email" binding:"omitempty,email,max=255"`
	Password string  `json:"password" binding:"omitempty,min=6,max=72"`
}

// UserResponse adalah representasi publik akun (tanpa password).
type UserResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthResponse dikembalikan saat register dan login.
type AuthResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"token_type"`
	ExpiresAt time.Time    `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// ToUserResponse memetakan model User menjadi UserResponse.
func ToUserResponse(u *database.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
