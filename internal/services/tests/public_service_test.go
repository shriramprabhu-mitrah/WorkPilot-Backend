package services_test

import (
	"testing"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type publicRepoStub struct {
	countries []models.Country
	country   models.Country
	err       *response.Error
}

func (s *publicRepoStub) GetCountries(name string) ([]models.Country, *response.Error) {
	return s.countries, s.err
}

func (s *publicRepoStub) GetCountryByID(id uuid.UUID) (models.Country, *response.Error) {
	return s.country, s.err
}

func TestPublicService_GetCountries(t *testing.T) {
	logger := zap.NewNop()
	repo := &publicRepoStub{
		countries: []models.Country{{Name: "India", ISO2: "IN", ISO3: "IND"}},
	}
	service := services.InitPublicService(repo, logger)

	countries, err := service.GetCountries("Ind")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(countries) != 1 {
		t.Fatalf("expected 1 country, got %d", len(countries))
	}
	if countries[0].Name != "India" {
		t.Fatalf("expected country name India, got %s", countries[0].Name)
	}
}

func TestPublicService_GetCountryByID(t *testing.T) {
	logger := zap.NewNop()
	id := uuid.Must(uuid.NewV4())
	repo := &publicRepoStub{
		country: models.Country{ID: id, Name: "United States", ISO2: "US", ISO3: "USA"},
	}
	service := services.InitPublicService(repo, logger)

	country, err := service.GetCountryByID(id)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if country.ID != id {
		t.Fatalf("expected country id %s, got %s", id, country.ID)
	}
}
