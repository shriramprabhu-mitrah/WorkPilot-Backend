package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/api/handlers"
	"github.com/ms-kanban-server/api/repositories"
	"github.com/ms-kanban-server/api/services"
	"github.com/ms-kanban-server/internals/configs"
	"gorm.io/gorm"
)

func SetupRoutes(r *gin.Engine, db *gorm.DB, config *configs.Config) {
	// initialize repositories
	repo := repositories.InitRepositories(db)

	// initialize services
	service := services.InitRepositories(repo)

	// initialize handlers
	handlers.InitHandler(service)

}
