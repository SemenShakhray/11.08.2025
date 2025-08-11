package service

import (
	"context"
	"downloader/internal/config"
	"downloader/internal/models"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

type Service struct {
	tasks         map[string]*models.Task
	activeTasks   map[string]struct{}
	completedTask map[string]struct{}
	cfg           *config.Config
	log           *slog.Logger
	mu            sync.Mutex
}

func NewService(cfg *config.Config, log *slog.Logger) *Service {
	return &Service{
		cfg:           cfg,
		log:           log,
		tasks:         make(map[string]*models.Task),
		activeTasks:   make(map[string]struct{}),
		completedTask: make(map[string]struct{}),
	}
}

func (s *Service) CreateTask(ctx context.Context) (*models.TaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.activeTasks) >= s.cfg.Conditions.MaxCountCurrentTask {
		s.log.Error("Server busy. Too many active tasks", slog.Any("active tusk", s.activeTasks), slog.Int("max", s.cfg.Conditions.MaxCountCurrentTask))

		return nil, fmt.Errorf("server busy. Too many active tasks. Active task %s", s.activeTasks)
	}

	taskID := uuid.New().String()
	s.tasks[taskID] = &models.Task{
		ID:       taskID,
		Status:   models.StatusPending,
		URLFiles: []string{},
	}

	s.activeTasks[taskID] = struct{}{}

	var activeID, completedID []string
	for id := range s.activeTasks {
		activeID = append(activeID, id)
	}

	for id := range s.completedTask {
		completedID = append(completedID, id)
	}

	return &models.TaskResponse{
		Task:           s.tasks[taskID],
		ActiveTasks:    activeID,
		CompletedTasks: completedID,
	}, nil
}

func (s *Service) AddURLs(ctx context.Context, taskID string, req models.AddURLsRequest) (*models.AddURLsResponse, error) {
	s.mu.Lock()

	task, ok := s.tasks[taskID]
	if !ok {
		s.log.Error("task not found", slog.String("taskID", taskID))

		s.mu.Unlock()
		return nil, fmt.Errorf("task with id %s not found", taskID)
	}

	if !(task.Status == models.StatusAddLinks || task.Status == models.StatusPending) {
		s.log.Warn("the maximum number of links has been added to the task",
			slog.String("taskID", taskID),
			slog.String("status", string(task.Status)))

		s.mu.Unlock()
		return nil, fmt.Errorf("the maximum number of links has been added to the task. The task is %s", task.Status)
	}

	var res models.AddURLsResponse

	for i, url := range req.URLs {
		err := s.checkURL(url)
		if err != nil {
			s.log.Warn("URL is failed", slog.String("url", url), slog.String("reason", err.Error()))

			res.InvalidURLs = append(res.InvalidURLs, models.InvalidURL{
				URL:    url,
				Reason: err.Error(),
			})
			continue
		}
		task.URLFiles = append(task.URLFiles, url)
		res.ValidURLs = append(res.ValidURLs, url)

		if len(task.URLFiles) == 1 {
			task.Status = models.StatusAddLinks

			s.log.Info("task status updated to add_links", slog.String("taskID", taskID))
		}

		if len(task.URLFiles) == s.cfg.Conditions.MaxFilesPerTask {
			s.log.Info("the maximum number of links has been added to the task", slog.Int("max", s.cfg.Conditions.MaxFilesPerTask))

			task.Status = models.StatusProcessing

			res.RejectedURLs = append(res.RejectedURLs, req.URLs[i+1:]...)

			id := task.ID
			urls := task.URLFiles
			s.mu.Unlock()
			go s.taskProcessing(id, urls)

			return &res, nil
		}
	}

	s.mu.Unlock()
	return &res, nil
}

func (s *Service) GetStatusTask(ctx context.Context, taskID string) (*models.TaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		s.log.Error("task not found", slog.String("taskID", taskID))

		return nil, fmt.Errorf("task with id %s not found", taskID)
	}

	var activeID, completedID []string
	for taskID := range s.activeTasks {
		activeID = append(activeID, taskID)
	}

	for id := range s.completedTask {
		completedID = append(completedID, id)
	}

	if task.Status == models.StatusCompleted || task.Status == models.StatusFailed {
		delete(s.activeTasks, taskID)

		var compl, act []string

		for taskID := range s.activeTasks {
			act = append(act, taskID)
		}
		if _, ok := s.completedTask[taskID]; !ok {
			s.completedTask[taskID] = struct{}{}
		}

		for id := range s.completedTask {
			compl = append(compl, id)
		}

		res := &models.TaskResponse{
			Task: &models.Task{
				ID:         taskID,
				Status:     task.Status,
				URLArchive: task.URLArchive,
				Errors:     task.Errors,
			},
			ActiveTasks:    act,
			CompletedTasks: compl,
		}

		return res, nil
	}

	return &models.TaskResponse{
		Task: &models.Task{
			ID:     taskID,
			Status: task.Status,
		},
		ActiveTasks:    activeID,
		CompletedTasks: completedID,
	}, nil
}
