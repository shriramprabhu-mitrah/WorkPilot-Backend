package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ms-kanban-server/drivers/redis"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

func InitPublicHandler(logger *zap.Logger, countryService services.PublicService) *PublicHandler {
	return &PublicHandler{
		logger:        logger,
		publicService: countryService,
	}
}

type PublicHandler struct {
	logger        *zap.Logger
	publicService services.PublicService

	countriesCache []models.Country
	cacheMux       sync.RWMutex
	cacheFetchedAt time.Time
}

// HealthHandler godoc
//
// @Summary      Health check endpoint
// @Description  Returns system health status and dependency checks (database, redis)
// @Tags         Public
// @Produce      json
// @Param        full query bool false "Include detailed dependency health checks"
// @Success      200 {object} map[string]interface{}
// @Failure      538 {object} map[string]interface{}
// @Router       /health [get]
func (h *PublicHandler) HealthHandler(deps models.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		timestamp := time.Now().UTC().Format(time.RFC3339)
		full := c.Query("full") == "true"

		if !full {
			c.JSON(http.StatusOK, gin.H{
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
		statusCode := http.StatusOK

		sqlDB, err := deps.Database.DB()
		if err != nil {
			dependencies["database"] = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		} else if err := sqlDB.Ping(); err != nil {
			dependencies["database"] = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		}

		if err := redis.PingRedis(deps.Redis); err != nil {
			dependencies["redis"] = "unhealthy"
			statusCode = http.StatusServiceUnavailable
		}

		status := "healthy"
		if statusCode != http.StatusOK {
			status = "unhealthy"
		}

		c.JSON(statusCode, gin.H{
			"status":       status,
			"version":      "v1",
			"timestamp":    timestamp,
			"dependencies": dependencies,
		})
	}
}

// GetAllCountries godoc
//
// @Summary      Get all countries
// @Description  Returns a list of countries (code + name)
// @Tags         Lookup
// @Produce      json
// @Success      200 {object} response.SuccessResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /countries [get]
func (h *PublicHandler) GetAllCountries(c *gin.Context) {
	nameFilter := strings.TrimSpace(c.Query("name"))

	// Load full list from cache or service.
	var countries []models.Country

	h.cacheMux.RLock()
	if time.Since(h.cacheFetchedAt) < 24*time.Hour && len(h.countriesCache) > 0 {
		countries = h.countriesCache
		h.cacheMux.RUnlock()
	} else {
		h.cacheMux.RUnlock()

		data, err := h.publicService.GetCountries("")
		if err != nil {
			c.JSON(http.StatusInternalServerError, &response.ErrorResponse{
				Success: false,
				Error: response.Error{
					Code:       response.ErrInternalServerError,
					StatusCode: http.StatusInternalServerError,
					Message:    fmt.Sprintf("Failed to retrieve countries: %s", err.Message),
				},
			})
			return
		}

		h.cacheMux.Lock()
		h.countriesCache = data
		h.cacheFetchedAt = time.Now()
		h.cacheMux.Unlock()

		countries = data
	}

	if nameFilter != "" {
		filtered := make([]models.Country, 0)
		nameFilter = strings.ToLower(nameFilter)

		for _, country := range countries {
			if strings.Contains(strings.ToLower(country.Name), nameFilter) {
				filtered = append(filtered, country)
			}
		}

		countries = filtered
	}

	c.JSON(http.StatusOK, &response.SuccessResponse{
		Success:    true,
		StatusCode: http.StatusOK,
		Message:    "Countries retrieved successfully",
		Data:       countries,
	})
}
