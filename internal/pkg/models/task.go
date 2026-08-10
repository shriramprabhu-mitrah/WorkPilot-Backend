package models

import (
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
	Key            string         `json:"key" gorm:"type:varchar(50);not null;uniqueIndex:idx_project_task_key"`
	SequenceNumber int            `json:"sequence_number" gorm:"type:integer;not null;index"`
	Title          string         `json:"title" gorm:"type:varchar(255);not null"`
	Description    string         `json:"description,omitempty" gorm:"type:text"`
	Type           string         `json:"type" gorm:"type:varchar(50);not null;default:'task'"`
	Priority       string         `json:"priority" gorm:"type:varchar(50);not null;default:'medium'"`
	Status         string         `json:"status" gorm:"type:varchar(50);not null;default:'todo'"`
	BlockedReason  string         `json:"blocked_reason,omitempty" gorm:"type:text"`
	AssigneeID     *uuid.UUID     `json:"assignee_id,omitempty" gorm:"type:uuid;index"`
	Assignee       *User          `json:"assignee,omitempty" gorm:"foreignKey:AssigneeID"`
	StoryPoints    int            `json:"story_points" gorm:"type:integer;not null;default:0"`
	DueDate        *time.Time     `json:"due_date,omitempty" gorm:"type:timestamptz"`
	EstimatedHours *float64       `json:"estimated_hours,omitempty" gorm:"type:numeric"`
	ActualHours    *float64       `json:"actual_hours,omitempty" gorm:"type:numeric"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
	Labels         []Label        `json:"labels,omitempty" gorm:"many2many:task_labels;"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID, err = uuid.NewV7()
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
}
