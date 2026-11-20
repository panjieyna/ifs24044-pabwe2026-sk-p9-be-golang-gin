package post

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"app/internal/middleware"
	"app/internal/response"
)

// PostHandler adalah controller endpoint /api/public/*, /api/posts/*, dan /api/admin/*.
type PostHandler struct {
	service PostService
}

// NewPostHandler membuat PostHandler.
func NewPostHandler(service PostService) *PostHandler {
	return &PostHandler{service: service}
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPostNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrTitleRequired), errors.Is(err, ErrContentRequired):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		log.Printf("post: %v", err)
		response.Error(c, http.StatusInternalServerError, "Internal server error")
	}
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "Invalid ID parameter")
		return 0, false
	}
	return uint(id), true
}

func currentUser(c *gin.Context) uint {
	return c.GetUint(middleware.CtxUserID)
}

// ---------- Publik ----------

// GetCategories: GET /api/public/categories
func (h *PostHandler) GetCategories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Categories retrieved successfully", h.service.Categories())
}

// GetPublicPosts: GET /api/public/posts?q=&category=
func (h *PostHandler) GetPublicPosts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		query = c.Query("search")
	}
	posts, err := h.service.ListPublic(PostFilter{Query: query, Category: c.Query("category")})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Posts retrieved successfully", posts)
}

// GetPublicPostBySlug: GET /api/public/posts/:slug
func (h *PostHandler) GetPublicPostBySlug(c *gin.Context) {
	post, err := h.service.GetPublicBySlug(c.Param("slug"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Post retrieved successfully", post)
}

// ---------- Penulis ----------

// GetMyPosts: GET /api/posts
func (h *PostHandler) GetMyPosts(c *gin.Context) {
	posts, err := h.service.ListMine(currentUser(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Posts retrieved successfully", posts)
}

// CreatePost: POST /api/posts
func (h *PostHandler) CreatePost(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationMessage(err))
		return
	}
	post, err := h.service.Create(currentUser(c), req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Post created successfully", post)
}

// GetStats: GET /api/posts/stats
func (h *PostHandler) GetStats(c *gin.Context) {
	stats, err := h.service.Stats(currentUser(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Post statistics retrieved successfully", stats)
}

// GetMyPost: GET /api/posts/:id
func (h *PostHandler) GetMyPost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	post, err := h.service.GetMine(currentUser(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Post retrieved successfully", post)
}

// UpdatePost: PUT /api/posts/:id
func (h *PostHandler) UpdatePost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationMessage(err))
		return
	}
	post, err := h.service.Update(currentUser(c), id, req)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Post updated successfully", post)
}

// TogglePublish: PATCH /api/posts/:id/toggle-publish
func (h *PostHandler) TogglePublish(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	post, err := h.service.TogglePublish(currentUser(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	message := "Post moved to draft"
	if post.IsPublished {
		message = "Post published successfully"
	}
	response.Success(c, http.StatusOK, message, post)
}

// DeletePost: DELETE /api/posts/:id
func (h *PostHandler) DeletePost(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(currentUser(c), id); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Post deleted successfully")
}

// ---------- Admin ----------

// AdminGetUsers: GET /api/admin/users
func (h *PostHandler) AdminGetUsers(c *gin.Context) {
	users, err := h.service.AdminListUsers()
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Users retrieved successfully", users)
}

// AdminGetPosts: GET /api/admin/posts
func (h *PostHandler) AdminGetPosts(c *gin.Context) {
	posts, err := h.service.AdminListPosts()
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Posts retrieved successfully", posts)
}

// AdminGetUserPosts: GET /api/admin/users/:id/posts
func (h *PostHandler) AdminGetUserPosts(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	posts, err := h.service.AdminListUserPosts(id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "User posts retrieved successfully", posts)
}
