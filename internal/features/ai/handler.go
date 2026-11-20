package ai

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"app/internal/response"
)

// AiHandler adalah controller endpoint POST /api/posts/ai-generate.
type AiHandler struct {
	service AiService
}

// NewAiHandler membuat AiHandler.
func NewAiHandler(service AiService) *AiHandler {
	return &AiHandler{service: service}
}

// GeneratePost menghasilkan draf artikel lengkap beserta sumber penelusuran.
func (h *AiHandler) GeneratePost(c *gin.Context) {
	var req GeneratePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, response.ValidationMessage(err))
		return
	}

	res, err := h.service.GeneratePost(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrTopicRequired) {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("ai: %v", err)
		response.Error(c, http.StatusInternalServerError, "Failed to generate article")
		return
	}

	message := "Article draft generated successfully"
	if res.Generator == GeneratorFallback {
		message = "AI service is unavailable; an outline draft was generated instead"
	}
	response.Success(c, http.StatusOK, message, res)
}
