// Package response menyediakan helper format JSON standar {status, message, data?}.
package response

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Success mengirim respons status "success". Kunci "data" hanya disertakan bila ada.
func Success(c *gin.Context, httpStatus int, message string, data ...any) {
	res := gin.H{"status": "success", "message": message}
	if len(data) > 0 && data[0] != nil {
		res["data"] = data[0]
	}
	c.JSON(httpStatus, res)
}

// Fail mengirim respons 4xx dengan status "fail".
func Fail(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, gin.H{"status": "fail", "message": message})
}

// Error mengirim respons 5xx dengan status "error".
func Error(c *gin.Context, httpStatus int, message string) {
	c.JSON(httpStatus, gin.H{"status": "error", "message": message})
}

// AbortFail menghentikan rantai middleware dengan respons 4xx.
func AbortFail(c *gin.Context, httpStatus int, message string) {
	c.AbortWithStatusJSON(httpStatus, gin.H{"status": "fail", "message": message})
}

// AbortError menghentikan rantai middleware dengan respons 5xx.
func AbortError(c *gin.Context, httpStatus int, message string) {
	c.AbortWithStatusJSON(httpStatus, gin.H{"status": "error", "message": message})
}

// ValidationMessage mengubah error binding/validator Gin menjadi pesan yang mudah dibaca.
func ValidationMessage(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return "Invalid request body"
	}
	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		field := toSnake(fe.Field())
		switch fe.Tag() {
		case "required":
			msgs = append(msgs, fmt.Sprintf("%s is required", field))
		case "email":
			msgs = append(msgs, fmt.Sprintf("%s must be a valid email address", field))
		case "min":
			msgs = append(msgs, fmt.Sprintf("%s must be at least %s characters", field, fe.Param()))
		case "max":
			msgs = append(msgs, fmt.Sprintf("%s must be at most %s characters", field, fe.Param()))
		default:
			msgs = append(msgs, fmt.Sprintf("%s is invalid", field))
		}
	}
	return "Validation failed: " + strings.Join(msgs, ", ")
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
