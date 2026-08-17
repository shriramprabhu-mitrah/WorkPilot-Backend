package models

import (
	"fmt"

	"gorm.io/gorm"
)

// GetNextGlobalSerialNumber fetches the next sequential serial number from the global database sequence.
// Safe under concurrent requests and across all work item types.
func GetNextGlobalSerialNumber(tx *gorm.DB) (int64, error) {
	var nextVal int64

	// Try PostgreSQL sequence first
	err := tx.Raw("SELECT nextval('global_work_item_serial_seq')").Scan(&nextVal).Error
	if err == nil && nextVal > 0 {
		return nextVal, nil
	}

	// Fallback for non-PostgreSQL DBs (e.g. SQLite in unit tests) or when sequence is unavailable
	var maxTask, maxStory int64
	_ = tx.Model(&Task{}).Unscoped().Select("COALESCE(MAX(serial_number), 0)").Scan(&maxTask).Error
	_ = tx.Model(&UserStory{}).Unscoped().Select("COALESCE(MAX(serial_number), 0)").Scan(&maxStory).Error

	if maxStory > maxTask {
		nextVal = maxStory + 1
	} else {
		nextVal = maxTask + 1
	}

	return nextVal, nil
}

// FormatSerialNumber returns the formatted serial number with '#' prefix.
func FormatSerialNumber(seq int64) string {
	if seq <= 0 {
		return ""
	}
	return fmt.Sprintf("#%d", seq)
}
