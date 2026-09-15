package response

import "github.com/gin-gonic/gin"

func Errorf(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}