package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"app/internal/database"
	"app/internal/middleware"
)

const secret = "auth-test-secret"

var errBoom = errors.New("boom")

func init() {
	gin.SetMode(gin.TestMode)
}

type envelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.ConnectTest()
	require.NoError(t, err)
	return db
}

func newRouter(svc AuthService) *gin.Engine {
	h := NewAuthHandler(svc)
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/logout", h.Logout)
	r.GET("/profile", middleware.Auth(secret), h.GetProfile)
	r.PUT("/profile", middleware.Auth(secret), h.UpdateProfile)
	return r
}

func call(t *testing.T, r *gin.Engine, method, path, body, token string) (int, envelope) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	var env envelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env), w.Body.String())
	return w.Code, env
}

func authData(t *testing.T, env envelope) AuthResponse {
	t.Helper()
	var res AuthResponse
	require.NoError(t, json.Unmarshal(env.Data, &res))
	return res
}

func tokenFor(t *testing.T, userID uint) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{
		UserID: userID, Role: "member",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	require.NoError(t, err)
	return tok
}

func TestRegisterLoginProfileFlow(t *testing.T) {
	r := newRouter(NewAuthService(NewUserRepository(newDB(t)), secret))

	code, env := call(t, r, http.MethodPost, "/register", `{"name":"Budi","username":"budi","email":"budi@example.com","password":"rahasia1"}`, "")
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "success", env.Status)
	reg := authData(t, env)
	assert.NotEmpty(t, reg.Token)
	assert.Equal(t, "Bearer", reg.TokenType)
	assert.Equal(t, "Budi", reg.User.Name)
	assert.Equal(t, "member", reg.User.Role)

	code, env = call(t, r, http.MethodPost, "/register", `{"username":"siti","email":"siti@example.com","password":"rahasia1","role":"Admin"}`, "")
	require.Equal(t, http.StatusCreated, code)
	siti := authData(t, env)
	assert.Equal(t, "siti", siti.User.Name)
	assert.Equal(t, "siti@example.com", siti.User.Email)
	assert.Equal(t, "admin", siti.User.Role)

	code, env = call(t, r, http.MethodPost, "/register", `{"username":"joko","email":"joko@example.com","password":"rahasia1","role":"superadmin"}`, "")
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "member", authData(t, env).User.Role)
	assert.NotContains(t, strings.ToLower(string(env.Data)), "password")

	for _, body := range []string{
		`{"username":"budi","password":"rahasia1"}`,
		`{"username":"BUDI@example.com","password":"rahasia1"}`,
		`{"email":"budi@example.com","password":"rahasia1"}`,
	} {
		code, env = call(t, r, http.MethodPost, "/login", body, "")
		assert.Equal(t, http.StatusOK, code, body)
		assert.NotEmpty(t, authData(t, env).Token)
	}

	code, _ = call(t, r, http.MethodPost, "/login", `{"username":"budi","password":"salah123"}`, "")
	assert.Equal(t, http.StatusUnauthorized, code)
	code, _ = call(t, r, http.MethodPost, "/login", `{"username":"ghost","password":"rahasia1"}`, "")
	assert.Equal(t, http.StatusUnauthorized, code)
	code, _ = call(t, r, http.MethodPost, "/login", `{"username":"  ","password":"rahasia1"}`, "")
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, r, http.MethodPost, "/login", `{"username":"budi"}`, "")
	assert.Equal(t, http.StatusBadRequest, code)

	code, env = call(t, r, http.MethodPost, "/logout", ``, "")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "success", env.Status)

	code, env = call(t, r, http.MethodGet, "/profile", ``, reg.Token)
	assert.Equal(t, http.StatusOK, code)
	var profile UserResponse
	require.NoError(t, json.Unmarshal(env.Data, &profile))
	assert.Equal(t, "budi", profile.Username)

	code, env = call(t, r, http.MethodPut, "/profile", `{"name":"Budi S.","email":"budi.s@example.com","password":"baru1234"}`, reg.Token)
	assert.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(env.Data, &profile))
	assert.Equal(t, "Budi S.", profile.Name)
	assert.Equal(t, "budi.s@example.com", profile.Email)

	code, _ = call(t, r, http.MethodPost, "/login", `{"username":"budi","password":"baru1234"}`, "")
	assert.Equal(t, http.StatusOK, code)

	code, env = call(t, r, http.MethodPut, "/profile", `{"name":"   ","email":"BUDI.S@example.com"}`, reg.Token)
	assert.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(env.Data, &profile))
	assert.Equal(t, "budi", profile.Name)

	code, _ = call(t, r, http.MethodPut, "/profile", `{"email":"siti@example.com"}`, reg.Token)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, r, http.MethodPut, "/profile", `{"email":"bukan-email"}`, reg.Token)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(t, r, http.MethodPut, "/profile", `{"password":""}`, reg.Token)
	assert.Equal(t, http.StatusOK, code)

	code, _ = call(t, r, http.MethodGet, "/profile", ``, tokenFor(t, 9999))
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, r, http.MethodPut, "/profile", `{}`, tokenFor(t, 9999))
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, r, http.MethodGet, "/profile", ``, "")
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestRegisterValidation(t *testing.T) {
	r := newRouter(NewAuthService(NewUserRepository(newDB(t)), secret))
	code, _ := call(t, r, http.MethodPost, "/register", `{"username":"andi","email":"andi@example.com","password":"rahasia1"}`, "")
	require.Equal(t, http.StatusCreated, code)

	cases := map[string]string{
		"invalid json":     `{`,
		"missing password": `{"username":"x1x","email":"x1x@example.com"}`,
		"missing email":    `{"username":"x2x","password":"rahasia1"}`,
		"short password":   `{"username":"xyz","email":"xyz@example.com","password":"123"}`,
		"invalid email":    `{"username":"xyz","email":"nope","password":"rahasia1"}`,
		"invalid username": `{"username":"bad name!","email":"bad@example.com","password":"rahasia1"}`,
		"taken username":   `{"username":"ANDI","email":"andi.lain@example.com","password":"rahasia1"}`,
		"taken email":      `{"username":"andi2","email":"Andi@example.com","password":"rahasia1"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			code, env := call(t, r, http.MethodPost, "/register", body, "")
			assert.Equal(t, http.StatusBadRequest, code)
			assert.Equal(t, "fail", env.Status)
		})
	}
}

// faultyRepo membungkus repository asli dan menggagalkan satu operasi tertentu.
type faultyRepo struct {
	UserRepository
	failOn string
}

func (f *faultyRepo) err(op string) error {
	if f.failOn == op {
		return errBoom
	}
	return nil
}

func (f *faultyRepo) FindByUsernameOrEmail(id string) (*database.User, error) {
	if e := f.err("find"); e != nil {
		return nil, e
	}
	return f.UserRepository.FindByUsernameOrEmail(id)
}

func (f *faultyRepo) FindByID(id uint) (*database.User, error) {
	if e := f.err("findByID"); e != nil {
		return nil, e
	}
	return f.UserRepository.FindByID(id)
}

func (f *faultyRepo) ExistsByUsername(u string) (bool, error) {
	if e := f.err("existsUsername"); e != nil {
		return false, e
	}
	return f.UserRepository.ExistsByUsername(u)
}

func (f *faultyRepo) ExistsByEmail(e string, id uint) (bool, error) {
	if err := f.err("existsEmail"); err != nil {
		return false, err
	}
	return f.UserRepository.ExistsByEmail(e, id)
}

func (f *faultyRepo) Create(u *database.User) error {
	if e := f.err("create"); e != nil {
		return e
	}
	return f.UserRepository.Create(u)
}

func (f *faultyRepo) Update(u *database.User) error {
	if e := f.err("update"); e != nil {
		return e
	}
	return f.UserRepository.Update(u)
}

func seededUser(t *testing.T, repo UserRepository) *database.User {
	t.Helper()
	hashed, err := database.HashPassword("rahasia1")
	require.NoError(t, err)
	u := &database.User{Name: "Dewi", Username: "dewi", Email: "dewi@example.com", Password: hashed, Role: "member"}
	require.NoError(t, repo.Create(u))
	return u
}

func TestServiceErrorPaths(t *testing.T) {
	newSvc := func(t *testing.T, failOn string) (AuthService, *database.User) {
		base := NewUserRepository(newDB(t))
		u := seededUser(t, base)
		return NewAuthService(&faultyRepo{UserRepository: base, failOn: failOn}, secret), u
	}
	reg := RegisterRequest{Username: "baru", Email: "baru@example.com", Password: "rahasia1"}
	newEmail := "lain@example.com"
	newPass := "rahasia2"

	svc, _ := newSvc(t, "existsUsername")
	_, err := svc.Register(reg)
	assert.ErrorIs(t, err, errBoom)

	svc, _ = newSvc(t, "existsEmail")
	_, err = svc.Register(reg)
	assert.ErrorIs(t, err, errBoom)

	svc, _ = newSvc(t, "create")
	_, err = svc.Register(reg)
	assert.ErrorIs(t, err, errBoom)

	svc, _ = newSvc(t, "")
	_, err = svc.Register(RegisterRequest{Username: "panjang", Email: "panjang@example.com", Password: strings.Repeat("p", 80)})
	assert.Error(t, err)

	svc, _ = newSvc(t, "find")
	_, err = svc.Login(LoginRequest{Username: "dewi", Password: "rahasia1"})
	assert.ErrorIs(t, err, errBoom)

	svc, u := newSvc(t, "findByID")
	_, err = svc.GetProfile(u.ID)
	assert.ErrorIs(t, err, errBoom)

	svc, u = newSvc(t, "existsEmail")
	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{Email: newEmail})
	assert.ErrorIs(t, err, errBoom)

	svc, u = newSvc(t, "update")
	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{Password: newPass})
	assert.ErrorIs(t, err, errBoom)

	svc, u = newSvc(t, "")
	tooLong := strings.Repeat("p", 80)
	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{Password: tooLong})
	assert.Error(t, err)
}

func TestHandlerInternalError(t *testing.T) {
	base := NewUserRepository(newDB(t))
	r := newRouter(NewAuthService(&faultyRepo{UserRepository: base, failOn: "existsUsername"}, secret))
	code, env := call(t, r, http.MethodPost, "/register", `{"username":"budi","email":"budi@example.com","password":"rahasia1"}`, "")
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error", env.Status)
	assert.Equal(t, "Internal server error", env.Message)
}
