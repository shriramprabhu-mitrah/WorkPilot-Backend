package models

import (
	"time"

	"github.com/gofrs/uuid"
)

type Country struct {
	ID        uuid.UUID `json:"id" gorm:"primaryKey;type:uuid"`
	Name      string    `json:"name" gorm:"size:100;not null"`
	ISO2      string    `json:"iso2" gorm:"size:2;not null;uniqueIndex"`
	ISO3      string    `json:"iso3" gorm:"size:3;not null;uniqueIndex"`
	PhoneCode string    `json:"phone_code" gorm:"size:10"`
	Timezone  []string  `json:"timezone" gorm:"not null"`
	FlagEmoji string    `json:"flag_emoji" gorm:"size:10"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;type:timestamptz"`
	UpdatedAt time.Time `json:"updated_at" gorm:"type:timestamptz"`
}
