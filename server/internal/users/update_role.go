package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UpdateRole(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpdateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		id := c.Param("id")

		target, err := repo.GetByID(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if target.Role == RoleOwner && req.Role != RoleOwner {
			owners, err := repo.CountOwners(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			if owners <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cannot demote the last owner"})
				return
			}
		}

		user, err := repo.UpdateRole(c.Request.Context(), id, req.Role)
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
