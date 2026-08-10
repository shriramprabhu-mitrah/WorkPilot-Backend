package models

import (
	"time"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

type OrphanedFile struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	StoragePath   string     `json:"storage_path" gorm:"type:text;not null"`
	Attempts      int        `json:"attempts" gorm:"type:integer;not null;default:0"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty" gorm:"type:timestamptz"`
	LastError     string     `json:"last_error,omitempty" gorm:"type:text"`
	ClaimedUntil  *time.Time `json:"claimed_until,omitempty" gorm:"type:timestamptz"`
	CreatedAt     time.Time  `json:"created_at" gorm:"type:timestamptz;not null"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty" gorm:"type:timestamptz"`
}

func (o *OrphanedFile) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == uuid.Nil {
		o.ID, err = uuid.NewV7()
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	return
}
