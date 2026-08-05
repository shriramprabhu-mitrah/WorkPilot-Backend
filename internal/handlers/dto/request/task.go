package request

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusInReview   TaskStatus = "in_review"
	TaskStatusTesting    TaskStatus = "testing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusBlocked    TaskStatus = "blocked"
)

func (r TaskStatus) Validate() error {
	switch r {
	case TaskStatusTodo,
		TaskStatusInProgress,
		TaskStatusInReview,
		TaskStatusTesting,
		TaskStatusCompleted,
		TaskStatusBlocked:
		return nil
	default:
		return fmt.Errorf("Invalid task status: %s", r)
	}
}

type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

func (r TaskPriority) Validate() error {
	switch r {
	case TaskPriorityLow,
		TaskPriorityMedium,
		TaskPriorityHigh,
		TaskPriorityCritical:
		return nil
	default:
		return fmt.Errorf("Invalid task priority: %s", r)
	}
}

type TaskType string

const (
	TaskTypeBug     TaskType = "bug"
	TaskTypeFeature TaskType = "feature"
	TaskTypeTask    TaskType = "task"
	TaskTypeChore   TaskType = "chore"
	TaskTypeStory   TaskType = "story"
)

func (r TaskType) Validate() error {
	switch r {
	case TaskTypeBug,
		TaskTypeFeature,
		TaskTypeTask,
		TaskTypeChore,
		TaskTypeStory:
		return nil
	default:
		return fmt.Errorf("Invalid task type: %s", r)
	}
}

type CreateTaskRequest struct {
	Title          string     `json:"title" binding:"required,min=3,max=255"`
	Description    string     `json:"description"`
	Type           string     `json:"type" binding:"required,oneof=bug feature task chore story"`
	Priority       string     `json:"priority" binding:"required,oneof=low medium high critical"`
	Status         string     `json:"status" binding:"omitempty,oneof=todo in_progress in_review testing completed blocked"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	SprintID       *uuid.UUID `json:"sprint_id"`
	StoryPoints    int        `json:"story_points" binding:"min=0"`
	DueDate        *time.Time `json:"due_date"`
	EstimatedHours *float64   `json:"estimated_hours" binding:"omitempty,min=0"`
	ActualHours    *float64   `json:"actual_hours" binding:"omitempty,min=0"`
	ProjectID      uuid.UUID  `json:"-"`
	UserID         uuid.UUID  `json:"-"`
	OrganizationID uuid.UUID  `json:"-"`
}

type UpdateTaskRequest struct {
	Title          *string    `json:"title" binding:"omitempty,min=3,max=255"`
	Description    *string    `json:"description"`
	Type           *string    `json:"type" binding:"omitempty,oneof=bug feature task chore story"`
	Priority       *string    `json:"priority" binding:"omitempty,oneof=low medium high critical"`
	Status         *string    `json:"status" binding:"omitempty,oneof=todo in_progress in_review testing completed blocked"`
	BlockedReason  *string    `json:"blocked_reason"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	SprintID       *uuid.UUID `json:"sprint_id"`
	StoryPoints    *int       `json:"story_points" binding:"omitempty,min=0"`
	DueDate        *time.Time `json:"due_date"`
	EstimatedHours *float64   `json:"estimated_hours" binding:"omitempty,min=0"`
	ActualHours    *float64   `json:"actual_hours" binding:"omitempty,min=0"`
	TaskID         uuid.UUID  `json:"-"`
	ProjectID      uuid.UUID  `json:"-"`
	UserID         uuid.UUID  `json:"-"`
	OrganizationID uuid.UUID  `json:"-"`
}

type CloneTaskRequest struct {
	KeepAssignee   bool      `json:"keep_assignee"`
	TaskID         uuid.UUID `json:"-"`
	ProjectID      uuid.UUID `json:"-"`
	UserID         uuid.UUID `json:"-"`
	OrganizationID uuid.UUID `json:"-"`
}

type TaskFilter struct {
	response.PaginationQuery
	response.SortQuery
	Status    string `form:"status"`
	Assignee  string `form:"assignee_id"`
	Sprint    string `form:"sprint_id"`
	Search    string `form:"search"`
	Type      string `form:"type"`
	Priority  string `form:"priority"`
	IsDeleted bool   `form:"is_deleted"`
}

type BulkUpdateTaskItem struct {
	TaskID        uuid.UUID  `json:"task_id" binding:"required"`
	Status        *string    `json:"status" binding:"omitempty,oneof=todo in_progress in_review testing completed blocked"`
	BlockedReason *string    `json:"blocked_reason"`
	SprintID      *uuid.UUID `json:"sprint_id"`
	AssigneeID    *uuid.UUID `json:"assignee_id"`
}

type BulkUpdateTasksRequest struct {
	Tasks          []BulkUpdateTaskItem `json:"tasks" binding:"required,min=1,dive"`
	ProjectID      uuid.UUID            `json:"-"`
	UserID         uuid.UUID            `json:"-"`
	OrganizationID uuid.UUID            `json:"-"`
}
