package publicrepo

import (
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublicRepository interface {
	GetCountries(name string) ([]models.Country, *response.Error)
}

func InitPublicRepository(deps models.Config) PublicRepository {
	return &publicRepository{
		DB:     deps.Database,
		logger: deps.Logger,
	}
}

type publicRepository struct {
	DB     *gorm.DB
	logger *zap.Logger
}

func (r *publicRepository) GetCountries(name string) ([]models.Country, *response.Error) {
	var countries []models.Country
	query := r.DB.Table("countries")
	if name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}
	if err := query.Order("name ASC").Find(&countries).Error; err != nil {
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: 500,
			Message:    "Failed to retrieve countries",
		}
	}
	return countries, nil
}
