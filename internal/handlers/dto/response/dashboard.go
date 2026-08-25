package response

import "github.com/gofrs/uuid"

type DashboardOverview struct {
	TotalTasks int64 `json:"total_tasks"`
	Completed  int64 `json:"completed"`
	Pending    int64 `json:"pending"`
	Overdue    int64 `json:"overdue"`
	Duesoon    int64 `json:"due_soon"`
}

type TaskStatus struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type SprintBurndown struct {
	Day         int     `json:"day"`
	Date        string  `json:"date"`
	IdealHours  float64 `json:"ideal_hours"`
	ActualHours float64 `json:"actual_hours"`
}

type WeeklyProgress struct {
	Day       string `json:"day"`
	Planned   int64  `json:"planned"`
	Completed int    `json:"completed"`
}

type TeamWorkload struct {
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
	Color     string    `json:"color"`
	TaskCount int64     `json:"task_count"`
	Points    float64   `json:"points"`
}

type DashboardResponse struct {
	Overview       DashboardOverview `json:"overview"`
	TaskStatus     map[string]int64  `json:"task_status"`
	SprintBurndown []SprintBurndown  `json:"sprint_burndown"`
	// WeeklyProgress []WeeklyProgress  `json:"weekly_progress"`
	TeamWorkload []TeamWorkload `json:"team_workload"`
}

type DashboardSprintBurndownResponse struct {
	Sprints []SprintBurndownData `json:"sprints"`
}

type SprintBurndownData struct {
	SprintID   uuid.UUID        `json:"sprint_id"`
	SprintName string           `json:"sprint_name"`
	Burndown   []SprintBurndown `json:"burndown"`
}
