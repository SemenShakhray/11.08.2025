package handlers

import (
	"context"
	"downloader/internal/models"
	"log/slog"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Servicer
	Log     *slog.Logger
}

type Servicer interface {
	CreateTask(ctx context.Context) (*models.TaskResponse, error)
	AddURLs(ctx context.Context, taskID string, req models.AddURLsRequest) (*models.AddURLsResponse, error)
	GetStatusTask(ctx context.Context, taskID string) (*models.TaskResponse, error)
}

func NewHandler(srv Servicer, log *slog.Logger) *Handler {
	return &Handler{
		service: srv,
		Log:     log,
	}
}

func (h *Handler) CreateTask(c *gin.Context) {

	res, err := h.service.CreateTask(c.Request.Context())
	if err != nil {
		h.Log.Error("invalid request", slog.String("path", c.Request.URL.Path), slog.String("error", err.Error()))

		c.JSON(500, models.ErrorResponse{
			Request: c.Request.URL.Path,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

func (h *Handler) AddURLs(c *gin.Context) {
	taskID := c.Param("id")

	var urls models.AddURLsRequest
	if err := c.ShouldBindJSON(&urls); err != nil {
		h.Log.Error("invalid request", slog.String("path", c.Request.URL.Path), slog.String("error", err.Error()))

		c.JSON(400, models.ErrorResponse{
			Request: c.Request.URL.Path,
			Error:   err.Error(),
		})
		return
	}

	res, err := h.service.AddURLs(c.Request.Context(), taskID, urls)
	if err != nil {
		c.JSON(500, models.ErrorResponse{
			Request: c.Request.URL.Path,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(200, res)
}

func (h *Handler) GetStatusTask(c *gin.Context) {
	taskID := c.Param("id")

	res, err := h.service.GetStatusTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(500, models.ErrorResponse{
			Request: c.Request.URL.Path,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(200, res)
}
