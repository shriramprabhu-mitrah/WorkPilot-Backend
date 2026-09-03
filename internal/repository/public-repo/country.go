package publicrepo

import (
	"errors"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublicRepository interface {
	GetCountries(name string) ([]models.Country, *response.Error)
	GetCountryByID(id uuid.UUID) (models.Country, *response.Error)
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

func (r *publicRepository) GetCountryByID(id uuid.UUID) (models.Country, *response.Error) {
	var country models.Country
	if err := r.DB.Table("countries").Where("id = ?", id).First(&country).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Country{}, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Country not found",
			}
		}
		return models.Country{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to retrieve country",
		}
	}
	return country, nil
}
