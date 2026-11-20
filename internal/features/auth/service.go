package auth

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"app/internal/database"
	"app/internal/middleware"
)

// Error domain autentikasi.
var (
	ErrInvalidUsername    = errors.New("username may only contain letters, numbers, dots, underscores, and dashes")
	ErrIdentifierRequired = errors.New("username or email is required")
	ErrUsernameTaken      = errors.New("username already registered")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// TokenTTL adalah masa berlaku token JWT.
const TokenTTL = 24 * time.Hour

// AuthService mendefinisikan use case autentikasi.
type AuthService interface {
	Register(req RegisterRequest) (*AuthResponse, error)
	Login(req LoginRequest) (*AuthResponse, error)
	GetProfile(userID uint) (*UserResponse, error)
	UpdateProfile(userID uint, req UpdateProfileRequest) (*UserResponse, error)
}

type authService struct {
	repo   UserRepository
	secret string
}

// NewAuthService membuat AuthService dengan kunci penandatanganan JWT (APP_KEY).
func NewAuthService(repo UserRepository, secret string) AuthService {
	return &authService{repo: repo, secret: secret}
}

func (s *authService) Register(req RegisterRequest) (*AuthResponse, error) {
	username := strings.TrimSpace(req.Username)
	if !usernamePattern.MatchString(username) {
		return nil, ErrInvalidUsername
	}

	taken, err := s.repo.ExistsByUsername(username)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrUsernameTaken
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = strings.ToLower(username) + "@users.delcom.local"
	}
	taken, err = s.repo.ExistsByEmail(email, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrEmailTaken
	}

	hashed, err := database.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = username
	}

	user := &database.User{Name: name, Username: username, Email: email, Password: hashed, Role: "member"}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	return s.issue(user), nil
}

func (s *authService) Login(req LoginRequest) (*AuthResponse, error) {
	identifier := strings.TrimSpace(req.Username)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		return nil, ErrIdentifierRequired
	}

	user, err := s.repo.FindByUsernameOrEmail(identifier)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return s.issue(user), nil
}

func (s *authService) GetProfile(userID uint) (*UserResponse, error) {
	user, err := s.findUser(userID)
	if err != nil {
		return nil, err
	}
	res := ToUserResponse(user)
	return &res, nil
}

func (s *authService) UpdateProfile(userID uint, req UpdateProfileRequest) (*UserResponse, error) {
	user, err := s.findUser(userID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
		if user.Name == "" {
			user.Name = user.Username
		}
	}

	if email := strings.TrimSpace(req.Email); email != "" && !strings.EqualFold(email, user.Email) {
		taken, err := s.repo.ExistsByEmail(email, user.ID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, ErrEmailTaken
		}
		user.Email = email
	}

	if req.Password != "" {
		hashed, err := database.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		user.Password = hashed
	}

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}
	res := ToUserResponse(user)
	return &res, nil
}

func (s *authService) findUser(userID uint) (*database.User, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *authService) issue(user *database.User) *AuthResponse {
	now := time.Now()
	expiresAt := now.Add(TokenTTL)
	claims := &middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	// HS256 dengan kunci []byte tidak pernah gagal ditandatangani.
	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
	return &AuthResponse{Token: signed, TokenType: "Bearer", ExpiresAt: expiresAt, User: ToUserResponse(user)}
}
