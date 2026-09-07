package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/config"
	"github.com/jojianya/sweetspot247-backend/db"
)

func main() {
	cfg := config.Load()

	pool, err := db.Connect(cfg.DSN())
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer pool.Close()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		dbStatus := "connected"
		if err := pool.Ping(context.Background()); err != nil {
			dbStatus = "unreachable"
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "ok", "db": dbStatus})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": dbStatus})
	})

	log.Printf("server starting on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
