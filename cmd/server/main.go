package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/api/routes"
	"github.com/ms-kanban-server/internals/configs"
	"github.com/ms-kanban-server/internals/db"
)

func main() {
	config := configs.LoadEnv()
	dbConn := db.InitDB()

	router := gin.Default()

	routes.SetupRoutes(router, dbConn, config)

	fmt.Println("Server is running on port", config.HTTP.Port)
	router.Run(fmt.Sprintf(":%d", config.HTTP.Port))
}
