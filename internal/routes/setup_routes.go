package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/drivers/redis"
	"github.com/ms-kanban-server/internal/pkg/models"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRoutes(deps models.Config) {

	// Health endpoint for liveness checks and readiness validation
	deps.Router.GET("/health", func(c *gin.Context) {
		timestamp := time.Now().UTC().Format(time.RFC3339)
		full := c.Query("full") == "true"

		if !full {
			c.JSON(200, gin.H{
				"status":    "healthy",
				"version":   "v1",
				"timestamp": timestamp,
			})
			return
		}

		dependencies := map[string]string{
			"database": "healthy",
			"redis":    "healthy",
		}
		statusCode := 200

		// Check database connection
		sqlDB, err := deps.Database.DB()
		if err != nil {
			dependencies["database"] = "unhealthy"
			statusCode = 503
		} else if err := sqlDB.Ping(); err != nil {
			dependencies["database"] = "unhealthy"
			statusCode = 503
		}

		// Check Redis connection
		if err := redis.PingRedis(deps.Redis); err != nil {
			dependencies["redis"] = "unhealthy"
			statusCode = 503
		}

		status := "healthy"
		if statusCode != 200 {
			status = "unhealthy"
		}

		c.JSON(statusCode, gin.H{
			"status":       status,
			"version":      "v1",
			"timestamp":    timestamp,
			"dependencies": dependencies,
		})
	})

	// Swagger init
	deps.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//Creating the router api/v1
	api := deps.Router.Group("/api/v1")
	{
		AuthRoutes(deps, api)
		OrganizationRoutes(deps, api)
		ProjectRoutes(deps, api)
	}

}
