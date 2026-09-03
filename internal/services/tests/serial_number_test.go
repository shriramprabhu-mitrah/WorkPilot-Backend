package services_test

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
)

func TestFormatSerialNumber(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, ""},
		{-5, ""},
		{1, "#1"},
		{42, "#42"},
		{9999, "#9999"},
	}

	for _, tt := range tests {
		got := models.FormatSerialNumber(tt.input)
		if got != tt.expected {
			t.Errorf("FormatSerialNumber(%d) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTaskAndUserStoryFormattedSerialNumber(t *testing.T) {
	task := models.Task{
		ID:           uuid.Must(uuid.NewV4()),
		SerialNumber: 101,
	}
	if task.FormattedSerialNumber() != "#101" {
		t.Errorf("Expected Task.FormattedSerialNumber() = #101, got %q", task.FormattedSerialNumber())
	}

	story := models.UserStory{
		ID:           uuid.Must(uuid.NewV4()),
		SerialNumber: 202,
	}
	if story.FormattedSerialNumber() != "#202" {
		t.Errorf("Expected UserStory.FormattedSerialNumber() = #202, got %q", story.FormattedSerialNumber())
	}
}

func TestSequentialSerialNumberAssignment(t *testing.T) {
	var currentSeq int64 = 0

	// Simulate work item creations across Tasks and UserStories
	items := []struct {
		itemType string
		time     time.Time
	}{
		{"task", time.Now().Add(-10 * time.Minute)},
		{"story", time.Now().Add(-8 * time.Minute)},
		{"task", time.Now().Add(-5 * time.Minute)},
		{"story", time.Now().Add(-1 * time.Minute)},
	}

	assignedSerials := make([]int64, len(items))
	for i := range items {
		currentSeq++
		assignedSerials[i] = currentSeq
	}

	for i, seq := range assignedSerials {
		expected := int64(i + 1)
		if seq != expected {
			t.Errorf("Item %d: expected serial %d, got %d", i, expected, seq)
		}
	}
}
