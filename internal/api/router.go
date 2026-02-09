package api

import (
	"net/http"
	"strconv"

	"github.com/LJTian/TrendingHub/internal/storage"
	"github.com/gin-gonic/gin"
)

type Server struct {
	store *storage.Store
}

func NewServer(store *storage.Store) *Server {
	return &Server{store: store}
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", s.health)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/news", s.listNews)
	}
}

func (s *Server) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) listNews(c *gin.Context) {
	channel := c.Query("channel")
	sort := c.DefaultQuery("sort", "latest")
	if sort != "latest" && sort != "hot" {
		sort = "latest"
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	items, err := s.store.ListNews(channel, sort, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "ok",
		"message": "success",
		"data":    items,
	})
}

