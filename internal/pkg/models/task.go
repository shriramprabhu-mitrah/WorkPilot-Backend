package models

import (
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Task struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID      uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_project_task_key"`
	Project        Project        `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	SprintID       *uuid.UUID     `json:"sprint_id,omitempty" gorm:"type:uuid;index"`
	Sprint         *Sprint        `json:"sprint,omitempty" gorm:"foreignKey:SprintID"`
	UserStoryID    *uuid.UUID     `json:"user_story_id,omitempty" gorm:"type:uuid;index"`
	UserStory      *UserStory     `json:"user_story,omitempty" gorm:"foreignKey:UserStoryID"`
	Key            string         `json:"key" gorm:"type:varchar(50);not null;uniqueIndex:idx_project_task_key"`
	SequenceNumber int            `json:"sequence_number" gorm:"type:integer;not null;index"`
	SerialNumber   int64          `json:"serial_number" gorm:"type:bigint;uniqueIndex"`
	Title          string         `json:"title" gorm:"type:varchar(255);not null"`
	Description    string         `json:"description,omitempty" gorm:"type:text"`
	Type           string         `json:"type" gorm:"type:varchar(50);not null;default:'task'"`
	Priority       string         `json:"priority" gorm:"type:varchar(50);not null;default:'medium'"`
	StatusID       uuid.UUID      `json:"status_id" gorm:"type:uuid;not null;index"`
	Status         string         `json:"status" gorm:"type:varchar(50);not null;default:'todo'"`
	BlockedReason  string         `json:"blocked_reason,omitempty" gorm:"type:text"`
	AssigneeID     *uuid.UUID     `json:"assignee_id,omitempty" gorm:"type:uuid;index"`
	ReporterID     *uuid.UUID     `json:"reporter_id,omitempty" gorm:"type:uuid;index"`
	Assignee       *User          `json:"assignee,omitempty" gorm:"foreignKey:AssigneeID"`
	Reporter       *User          `json:"reporter,omitempty" gorm:"foreignKey:ReporterID"`
	StoryPoints    int            `json:"story_points" gorm:"type:integer;not null;default:0"`
	DueDate        *time.Time     `json:"due_date,omitempty" gorm:"type:timestamptz"`
	EstimatedHours *float64       `json:"estimated_hours,omitempty" gorm:"type:numeric"`
	ActualHours    *float64       `json:"actual_hours,omitempty" gorm:"type:numeric"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
	Labels         []Label        `json:"labels,omitempty" gorm:"many2many:task_labels;"`
}

func (t Task) FormattedSerialNumber() string {
	return FormatSerialNumber(t.SerialNumber)
}

func (t *Task) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	if t.SerialNumber == 0 {
		t.SerialNumber, err = GetNextGlobalSerialNumber(tx)
		if err != nil {
			return err
		}
	}
	return
}

type TaskAccessContext struct {
	TaskID         uuid.UUID `gorm:"column:task_id"`
	ProjectID      uuid.UUID `gorm:"column:project_id"`
	OrganizationID uuid.UUID `gorm:"column:organization_id"`
	TaskKey        string    `gorm:"column:task_key"`
}

var DefaultStatusColors = map[string]string{
	"todo":        "#808080",
	"in_progress": "#1E90FF",
	"in_review":   "#FF8C00",
	"testing":     "#8A2BE2",
	"completed":   "#228B22",
	"blocked":     "#DC143C",
}

var DefaultStatusIsFinal = map[string]bool{
	"todo":        false,
	"in_progress": false,
	"in_review":   false,
	"testing":     false,
	"completed":   true,
	"blocked":     false,
}

func NormalizeTaskStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.ReplaceAll(s, " ", "_")
}

func IsDefaultTaskStatus(status string) bool {
	_, exists := DefaultStatusColors[NormalizeTaskStatus(status)]
	return exists
}
