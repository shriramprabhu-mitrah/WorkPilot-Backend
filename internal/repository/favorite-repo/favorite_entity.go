package favoriterepo

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FavoriteRepository interface {
	AddFavorite(favorite *models.Favorite) *response.Error
	RemoveFavorite(userID uuid.UUID, itemType string, itemID uuid.UUID) (*models.Favorite, *response.Error)
	RemoveFavoriteByID(userID uuid.UUID, favoriteID uuid.UUID) (*models.Favorite, *response.Error)
	GetFavoritesByUserID(userID uuid.UUID, itemType string) ([]models.Favorite, *response.Error)
	GetFavoriteByUserAndItem(userID uuid.UUID, itemType string, itemID uuid.UUID) (*models.Favorite, *response.Error)
	IsFavorited(userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, *response.Error)
}

func InitFavoriteRepository(deps models.Config) FavoriteRepository {
	return &favoriteDatabase{
		db:          deps.Database,
		redisClient: deps.Redis,
		logger:      deps.Logger,
	}
}

type favoriteDatabase struct {
	db          *gorm.DB
	redisClient *redisclient.Client
	logger      *zap.Logger
}
