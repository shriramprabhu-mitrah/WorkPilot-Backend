package response

import (
	"github.com/gofrs/uuid"
)

type SearchResult struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"` // "task", "user_story", "project", "member"
	Title       string    `json:"title"`
	Key         string    `json:"key,omitempty"`         // TASK-1, US-1, slug, or username
	Description string    `json:"description,omitempty"` // task/project description, user email
	Status      string    `json:"status,omitempty"`
	Priority    string    `json:"priority,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	ProjectName string     `json:"project_name,omitempty"`
}

type GlobalSearchResponse struct {
	Tasks       []SearchResult `json:"tasks"`
	UserStories []SearchResult `json:"user_stories"`
	Projects    []SearchResult `json:"projects"`
	Members     []SearchResult `json:"members"`
	Sprints     []SearchResult `json:"sprints"`
}
