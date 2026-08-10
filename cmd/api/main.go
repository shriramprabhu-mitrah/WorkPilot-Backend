package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/ms-kanban-server/docs"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/config"
	"github.com/ms-kanban-server/drivers/postgres"
	"github.com/ms-kanban-server/drivers/redis"
	"github.com/ms-kanban-server/internal/pkg/logger"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/scheduler"
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

	// Create root lifecycle context
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	//Initialize the Gin router and set up routes
	router := gin.Default()

	//Getting dependences in config
	deps := models.Config{
		Database: dbConn,
		Router:   router,
		Redis:    redisClient,
		Logger:   Logger,
		Context:  rootCtx,
	}

	// Set up routes
	routes.SetupRoutes(deps)

	// Start background scheduler
	go scheduler.Start(dbConn, Logger)

	// Start the server gracefully
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.HTTP.Port),
		Handler: router,
	}

	go func() {
		Logger.Info("Server is running", zap.String("port", config.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			Logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	Logger.Info("Shutting down API server gracefully...")
	cancelRoot() // Cancel context to stop background workers

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		Logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	Logger.Info("Server exited cleanly")
}
