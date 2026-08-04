package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type Sprint struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID   uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;uniqueIndex:idx_project_sprint_name"`
	Project     Project        `json:"project,omitzero" gorm:"foreignKey:ProjectID"`
	Name        string         `json:"name" gorm:"type:varchar(100);not null;uniqueIndex:idx_project_sprint_name"`
	Goal        string         `json:"goal,omitempty" gorm:"type:varchar(500)"`
	Status      string         `json:"status" gorm:"type:varchar(20);default:'planning'"`
	StartDate   time.Time      `json:"start_date" gorm:"type:date;not null"`
	EndDate     time.Time      `json:"end_date" gorm:"type:date;not null"`
	Velocity    *int           `json:"velocity" gorm:"type:integer"`
	CreatedByID uuid.UUID      `json:"created_by_id" gorm:"type:uuid;not null"`
	CreatedBy   User           `json:"created_by,omitzero" gorm:"foreignKey:CreatedByID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (s *Sprint) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}

type Task struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID      uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;index"`
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
	AssigneeID     *uuid.UUID     `json:"assignee_id,omitempty" gorm:"type:uuid;index"`
	Assignee       *User          `json:"assignee,omitempty" gorm:"foreignKey:AssigneeID"`
	StoryPoints    int            `json:"story_points" gorm:"type:integer;not null;default:0"`
	DueDate        *time.Time     `json:"due_date,omitempty" gorm:"type:timestamptz"`
	EstimatedHours *float64       `json:"estimated_hours,omitempty" gorm:"type:numeric"`
	ActualHours    *float64       `json:"actual_hours,omitempty" gorm:"type:numeric"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
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

type SprintSnapshot struct {
	ID                   uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	SprintID             uuid.UUID `json:"sprint_id" gorm:"type:uuid;not null;index"`
	Sprint               Sprint    `json:"sprint,omitempty" gorm:"foreignKey:SprintID"`
	Date                 time.Time `json:"date" gorm:"type:date;not null;uniqueIndex:idx_sprint_snapshot_sprint_date"`
	TotalStoryPoints     int       `json:"total_story_points" gorm:"type:integer;not null;default:0"`
	RemainingStoryPoints int       `json:"remaining_story_points" gorm:"type:integer;not null;default:0"`
	CreatedAt            time.Time `json:"created_at"`
}

func (s *SprintSnapshot) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == uuid.Nil {
		s.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return
}
