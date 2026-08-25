package routes

import (
	"github.com/ms-kanban-server/internal/pkg/models"
)

func SetupRoutes(deps models.Config) {

	//Creating the router api/v1
	api := deps.Router.Group("/api/v1")
	{
		AuditRoutes(deps, api)
		AuthRoutes(deps, api)
		OrganizationRoutes(deps, api)
		ProjectRoutes(deps, api)
		PublicRoutes(deps, api)
		SprintRoutes(deps, api)
		TaskRoutes(deps, api)
		UserStoryRoutes(deps, api)
		LabelRoutes(deps, api)
		CommentsRoutes(deps, api)
		CustomStatusRoutes(deps, api)
		UserStoryStatusRoutes(deps, api)
		WorkItemRoutes(deps, api)
		DashboardRoutes(deps, api)
		FavoriteRoutes(deps, api)
	}

}
