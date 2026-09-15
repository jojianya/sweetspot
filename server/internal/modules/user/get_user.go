package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetUser(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := repo.GetByID(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, user.ToPublic())
	}
}
