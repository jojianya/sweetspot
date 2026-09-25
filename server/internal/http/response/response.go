package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSON writes a JSON response body with the given status code.
func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

// OK writes a 200 response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Created writes a 201 response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

// NoContent writes an empty 204 response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error writes a {"error": message} body with the given status code.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

// BadRequest writes a 400 response.
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized writes a 401 response.
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden writes a 403 response.
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound writes a 404 response.
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// Conflict writes a 409 response.
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, message)
}

// TooManyRequests writes a 429 response.
func TooManyRequests(c *gin.Context, message string) {
	Error(c, http.StatusTooManyRequests, message)
}

// AbortError writes a {"error": message} body and aborts the request. It is for
// middleware that must stop handler execution (auth, rate limits, role gates).
func AbortError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
