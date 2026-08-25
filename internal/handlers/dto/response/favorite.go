package response

import (
	"time"

	"github.com/gofrs/uuid"
)

type FavoriteResponse struct {
	ID             uuid.UUID          `json:"id"`
	UserID         uuid.UUID          `json:"user_id"`
	ItemType       string             `json:"item_type"`
	UserStoryID    *uuid.UUID         `json:"user_story_id,omitempty"`
	TaskID         *uuid.UUID         `json:"task_id,omitempty"`
	ProjectID      *uuid.UUID         `json:"project_id,omitempty"`
	ProjectName    string             `json:"project_name,omitempty"`
	UserStoryName  string             `json:"user_story_name,omitempty"`
	UserStoryTitle string             `json:"user_story_title,omitempty"`
	TaskName       string             `json:"task_name,omitempty"`
	TaskTitle      string             `json:"task_title,omitempty"`
	UserStory      *UserStoryResponse `json:"user_story,omitempty"`
	Task           *TaskResponse      `json:"task,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
}

type FavoriteListResponse struct {
	Favorites        []FavoriteResponse `json:"favorites,omitempty"`
	Total            int64              `json:"total"`
	TotalUserStories int64              `json:"total_user_stories"`
	TotalTasks       int64              `json:"total_tasks"`
}

type RemoveFavoriteResponse struct {
	ID uuid.UUID `json:"id"`
}
