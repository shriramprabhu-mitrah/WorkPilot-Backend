package routes

import (
	"github.com/ms-kanban-server/internal/pkg/models"
)

func SetupRoutes(deps models.Config) {

	// Register open/public non-auth routes (health, swagger)
	PublicRoutes(deps)

	//Creating the router api/v1
	api := deps.Router.Group("/api/v1")
	{
		AuthRoutes(deps, api)
		OrganizationRoutes(deps, api)
		ProjectRoutes(deps, api)
	}

}
