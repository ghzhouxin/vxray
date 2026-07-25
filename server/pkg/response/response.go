package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: data})
}

func SuccessMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: message})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, APIResponse{Code: 0, Data: data})
}

func errorResponse(c *gin.Context, status int, message string) {
	c.JSON(status, APIResponse{Code: status, Message: message})
}

func BadRequest(c *gin.Context, message string) {
	errorResponse(c, http.StatusBadRequest, message)
}

func NotFound(c *gin.Context, message string) {
	errorResponse(c, http.StatusNotFound, message)
}

func Conflict(c *gin.Context, message string, data any) {
	c.JSON(http.StatusConflict, APIResponse{Code: http.StatusConflict, Message: message, Data: data})
}

func InternalError(c *gin.Context, message string) {
	errorResponse(c, http.StatusInternalServerError, message)
}
