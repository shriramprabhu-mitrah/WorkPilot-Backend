package request

import (
	"encoding/json"
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
	Title          string      `json:"title" binding:"required,min=3,max=255"`
	Description    string      `json:"description"`
	Type           string      `json:"type" binding:"required,oneof=bug feature task chore story"`
	Priority       string      `json:"priority" binding:"required,oneof=low medium high critical"`
	StatusID       *uuid.UUID  `json:"status_id" binding:"omitempty"`
	Status         string      `json:"status" binding:"omitempty"`
	AssigneeID     *uuid.UUID  `json:"assignee_id"`
	ReporterID     *uuid.UUID  `json:"reporter_id"`
	SprintID       *uuid.UUID  `json:"sprint_id"`
	UserStoryID    *uuid.UUID  `json:"user_story_id"`
	StoryPoints    int         `json:"story_points" binding:"min=0"`
	DueDate        *time.Time  `json:"due_date"`
	EstimatedHours *float64    `json:"estimated_hours" binding:"omitempty,min=0"`
	ActualHours    *float64    `json:"actual_hours" binding:"omitempty,min=0"`
	LabelIDs       []uuid.UUID `json:"label_ids"`
	AttachmentIDs  []uuid.UUID `json:"attachment_ids,omitempty"`
	ProjectID      uuid.UUID   `json:"-"`
	UserID         uuid.UUID   `json:"-"`
	OrganizationID uuid.UUID   `json:"-"`
}

func (r *CreateTaskRequest) UnmarshalJSON(data []byte) error {
	type Alias CreateTaskRequest
	var temp Alias
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	*r = CreateTaskRequest(temp)

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	if val, exists := rawMap["story_id"]; exists && r.UserStoryID == nil {
		if val != nil {
			if str, ok := val.(string); ok && str != "" {
				if id, err := uuid.FromString(str); err == nil {
					r.UserStoryID = &id
				}
			}
		}
	}
	return nil
}

type UpdateTaskRequest struct {
	Title          *string         `json:"title" binding:"omitempty,min=3,max=255"`
	Description    *string         `json:"description"`
	Type           *string         `json:"type" binding:"omitempty,oneof=bug feature task chore story"`
	Priority       *string         `json:"priority" binding:"omitempty,oneof=low medium high critical"`
	StatusID       *uuid.UUID      `json:"status_id" binding:"omitempty"`
	Status         *string         `json:"status" binding:"omitempty"`
	AssigneeID     *uuid.UUID      `json:"assignee_id"`
	ReporterID     *uuid.UUID      `json:"reporter_id"`
	SprintID       *uuid.UUID      `json:"sprint_id"`
	UserStoryID    *uuid.UUID      `json:"user_story_id"`
	StoryPoints    *int            `json:"story_points" binding:"omitempty,min=0"`
	DueDate        *time.Time      `json:"due_date"`
	EstimatedHours *float64        `json:"estimated_hours" binding:"omitempty,min=0"`
	ActualHours    *float64        `json:"actual_hours" binding:"omitempty,min=0"`
	LabelIDs       *[]uuid.UUID    `json:"label_ids"`
	TaskID         uuid.UUID       `json:"-"`
	ProjectID      uuid.UUID       `json:"-"`
	UserID         uuid.UUID       `json:"-"`
	OrganizationID uuid.UUID       `json:"-"`
	IsNullFields   map[string]bool `json:"-"`
}

func (r *UpdateTaskRequest) UnmarshalJSON(data []byte) error {
	type Alias UpdateTaskRequest
	var temp Alias
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	*r = UpdateTaskRequest(temp)

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	r.IsNullFields = make(map[string]bool)
	nullableFields := []string{"sprint_id", "assignee_id", "user_story_id", "due_date", "estimated_hours", "actual_hours"}
	for _, field := range nullableFields {
		if val, exists := rawMap[field]; exists && val == nil {
			r.IsNullFields[field] = true
		}
	}

	if val, exists := rawMap["story_id"]; exists {
		if val == nil {
			r.UserStoryID = nil
			r.IsNullFields["user_story_id"] = true
		} else if r.UserStoryID == nil {
			if str, ok := val.(string); ok && str != "" {
				if id, err := uuid.FromString(str); err == nil {
					r.UserStoryID = &id
				}
			}
		}
	}

	return nil
}

func (r *UpdateTaskRequest) IsSprintIDNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["sprint_id"]
}

func (r *UpdateTaskRequest) IsAssigneeIDNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["assignee_id"]
}

func (r *UpdateTaskRequest) IsUserStoryIDNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["user_story_id"]
}

func (r *UpdateTaskRequest) IsDueDateNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["due_date"]
}

func (r *UpdateTaskRequest) IsEstimatedHoursNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["estimated_hours"]
}

func (r *UpdateTaskRequest) IsActualHoursNull() bool {
	return r.IsNullFields != nil && r.IsNullFields["actual_hours"]
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
	StatusID       []string    `form:"status_id"`
	Assignee       []string    `form:"assignee_id"`
	Reporter       []string    `form:"reporter_id"`
	Sprint         []string    `form:"sprint_id"`
	UserStory      []string    `form:"user_story_id"`
	Search         string      `form:"search"`
	Type           []string    `form:"type"`
	Priority       []string    `form:"priority"`
	IsDeleted      bool        `form:"is_deleted"`
	Labels         []string    `form:"labels"`
	Match          string      `form:"match"`
	SequenceNumber *int        `form:"sequence_number"`
	SerialNumber   *int64      `form:"serial_number"`
	UnassignedTask bool        `form:"unassigned_task"`
	StatusIDs      []uuid.UUID `form:"-"` // Internal resolved status IDs
}

type BulkUpdateTaskItem struct {
	TaskID        uuid.UUID  `json:"task_id" binding:"required"`
	StatusID      *uuid.UUID `json:"status_id" binding:"omitempty"`
	Status        *string    `json:"status" binding:"omitempty"`
	SprintID      *uuid.UUID `json:"sprint_id"`
	AssigneeID    *uuid.UUID `json:"assignee_id"`
}

type BulkUpdateTasksRequest struct {
	Tasks          []BulkUpdateTaskItem `json:"tasks" binding:"required,min=1,dive"`
	ProjectID      uuid.UUID            `json:"-"`
	UserID         uuid.UUID            `json:"-"`
	OrganizationID uuid.UUID            `json:"-"`
}

type BulkDeleteTasksRequest struct {
	TaskIDs        []uuid.UUID `json:"task_ids" binding:"required,min=1,dive"`
	ProjectID      uuid.UUID   `json:"-"`
	UserID         uuid.UUID   `json:"-"`
	OrganizationID uuid.UUID   `json:"-"`
}
