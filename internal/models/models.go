package models

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusAddLinks   TaskStatus = "add_links"
	StatusProcessing TaskStatus = "processing"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

type Task struct {
	ID         string     `json:"id"`
	Status     TaskStatus `json:"status"`
	URLArchive string     `json:"url_archive,omitempty"`
	Errors     []string   `json:"errors,omitempty"`
	URLFiles   []string   `json:"url_files,omitempty"`
}

type TaskResponse struct {
	Task           *Task    `json:"task"`
	ActiveTasks    []string `json:"active_tasks"`
	CompletedTasks []string `json:"completed_tasks"`
}

type ErrorResponse struct {
	Request string `json:"request"`
	Error   string `json:"error"`
}

type AddURLsRequest struct {
	URLs []string `json:"urls"`
}

type AddURLsResponse struct {
	ValidURLs    []string     `json:"valid_urls,omitempty"`
	InvalidURLs  []InvalidURL `json:"invalid_urls,omitempty"`
	RejectedURLs []string     `json:"rejected_urls,omitempty"`
}

type InvalidURL struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}
