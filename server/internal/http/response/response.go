package response

import "github.com/gin-gonic/gin"

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}
