package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type UserStory struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey"`
	ProjectID    uuid.UUID      `json:"project_id" gorm:"type:uuid;not null;index"`
	Project      Project        `json:"project,omitempty" gorm:"foreignKey:ProjectID"`
	SprintID     *uuid.UUID     `json:"sprint_id,omitempty" gorm:"type:uuid;index"`
	Sprint       *Sprint        `json:"sprint,omitempty" gorm:"foreignKey:SprintID"`
	SerialNumber int64          `json:"serial_number" gorm:"type:bigint;uniqueIndex"`
	Title        string         `json:"title" gorm:"type:varchar(255);not null"`
	Description  string         `json:"description,omitempty" gorm:"type:text"`
	Priority     string         `json:"priority" gorm:"type:varchar(50);not null;default:'medium'"`
	StatusID     uuid.UUID      `json:"status_id" gorm:"type:uuid;not null;index"`
	Status       string         `json:"status" gorm:"type:varchar(50);not null;default:'todo'"`
	IsClosed     bool           `json:"is_closed" gorm:"type:boolean;not null;default:false"`
	StoryPoints  int            `json:"story_points" gorm:"type:integer;not null;default:0"`
	BacklogOrder int            `json:"backlog_order" gorm:"type:integer;not null;default:0"`
	AssigneeID   *uuid.UUID     `json:"assignee_id,omitempty" gorm:"type:uuid;index"`
	ReporterID   uuid.UUID      `json:"reporter_id" gorm:"type:uuid;not null;index"`
	Assignee     *User          `json:"assignee,omitempty" gorm:"foreignKey:AssigneeID"`
	Reporter     User           `json:"reporter" gorm:"foreignKey:ReporterID"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

func (u UserStory) FormattedSerialNumber() string {
	return FormatSerialNumber(u.SerialNumber)
}

type StoryTaskStats struct {
	UserStoryID uuid.UUID
	TotalTasks  int64
	Completed   int64
}

func (u *UserStory) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	if u.SerialNumber == 0 {
		u.SerialNumber, err = GetNextGlobalSerialNumber(tx)
		if err != nil {
			return err
		}
	}
	return
}

type UserStoryAccessContext struct {
	UserStoryID    uuid.UUID `gorm:"column:user_story_id"`
	ProjectID      uuid.UUID `gorm:"column:project_id"`
	OrganizationID uuid.UUID `gorm:"column:organization_id"`
	Title          string    `gorm:"column:title"`
}
