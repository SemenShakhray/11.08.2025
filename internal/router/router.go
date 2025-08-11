package router

import (
	"os"

	"downloader/internal/handlers"

	"github.com/gin-gonic/gin"
)

func NewRouter(h *handlers.Handler) *gin.Engine {
	r := gin.Default()

	r.Static("/archives", os.TempDir())

	r.POST("task", h.CreateTask)
	r.POST("task/:id", h.AddURLs)
	r.GET("task/:id", h.GetStatusTask)
	return r
}
