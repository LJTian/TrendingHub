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
		v1.GET("/news/dates", s.listNewsDates) // 必须放在 /news 前，避免被前缀匹配
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
	date := c.Query("date") // 可选，格式 2006-01-02，按日期展示

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := s.store.ListNews(channel, sort, limit, date)
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

func (s *Server) listNewsDates(c *gin.Context) {
	channel := c.Query("channel")
	limitStr := c.DefaultQuery("limit", "31")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 31
	}
	if limit > 365 {
		limit = 365
	}

	dates, err := s.store.ListPublishedDates(channel, limit)
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
		"data":    dates,
	})
}

