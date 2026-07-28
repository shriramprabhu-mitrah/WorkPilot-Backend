package services

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	publicrepo "github.com/ms-kanban-server/internal/repository/public-repo"
	"go.uber.org/zap"
)

type PublicService interface {
	GetCountries(name string) ([]models.Country, *response.Error)
	GetCountryByID(id uuid.UUID) (models.Country, *response.Error)
}

func InitPublicService(repo publicrepo.PublicRepository, logger *zap.Logger) PublicService {
	return &publicService{
		Repo:   repo,
		logger: logger,
	}
}

type publicService struct {
	Repo   publicrepo.PublicRepository
	logger *zap.Logger
}

func (s *publicService) GetCountries(name string) ([]models.Country, *response.Error) {
	countries, err := s.Repo.GetCountries(name)
	if err != nil {
		s.logger.Error("failed to retrieve countries from repository", zap.String("message", err.Message))
		return nil, err
	}

	return countries, nil
}

func (s *publicService) GetCountryByID(id uuid.UUID) (models.Country, *response.Error) {
	country, err := s.Repo.GetCountryByID(id)
	if err != nil {
		s.logger.Error("failed to retrieve country by id", zap.String("country_id", id.String()), zap.String("message", err.Message))
		return models.Country{}, err
	}

	return country, nil
}
