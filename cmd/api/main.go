package main

import (
	"fmt"
	"log"

	_ "github.com/ms-kanban-server/docs"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/drivers/postgres"
	"github.com/ms-kanban-server/drivers/redis"
	"github.com/ms-kanban-server/internal/pkg/logger"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/routes"
	"go.uber.org/zap"
)

// @title PMT API
// @version 1.0
// @description PMT Backend API
// @termsOfService http://swagger.io/terms/

// @contact.name PMT Team
// @contact.email olivergot26@gmail.com

// @license.name MIT

// @host localhost:6369
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {

	// Load configuration, initialize database connection, set up routes, and start the server
	config := config.LoadEnv()

	// Initialize the logger
	Logger, err := logger.InitLogger(config)
	if err != nil {
		log.Fatal("Failed to initialize logger :", err)
	} else {
		Logger.Info("Initialized the Logger")
	}

	// Initialize the database connection
	dbConn, err := postgres.InitDB(config)
	if err != nil {
		Logger.Fatal("Failed to initialize database connection :",
			zap.String("ERROR", err.Error()))
	} else {
		Logger.Info("Initialized the Database Connection",
			zap.String("port", config.Database.Port))
	}

	// Initialize the RedisClient
	redisClient, err := redis.InitRedisClient(config)
	if err != nil {
		Logger.Fatal("Failed to initialize Redis client:",
			zap.String("ERROR", err.Error()))
	} else {
		Logger.Info("Initialized the RedisClient",
			zap.String("port", config.Redis.Port))
	}

	//Initialize the Gin router and set up routes
	router := gin.Default()

	//Getting dependences in config
	deps := models.Config{
		Database: dbConn,
		Router:   router,
		Redis:    redisClient,
		Logger:   Logger,
	}

	// Set up routes
	routes.SetupRoutes(deps)

	// Start the server
	Logger.Info("Server is running",
		zap.String("port", config.HTTP.Port))
	err = router.Run(fmt.Sprintf(":%s", config.HTTP.Port))
	if err != nil {
		Logger.Fatal("Failed to start server ",
			zap.String("ERROR", err.Error()))
	}
}
