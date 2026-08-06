package services_test

import (
	"net/http"
	"testing"

	"github.com/gofrs/uuid"
	dto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/services"
	"go.uber.org/zap"
)

type stubLabelRepo struct {
	labels    map[uuid.UUID]*models.Label
	createErr *response.Error
	getErr    *response.Error
	updateErr *response.Error
	deleteErr *response.Error
}

func (s *stubLabelRepo) CreateLabel(label *models.Label) *response.Error {
	if s.createErr != nil {
		return s.createErr
	}
	if s.labels == nil {
		s.labels = make(map[uuid.UUID]*models.Label)
	}
	s.labels[label.ID] = label
	return nil
}

func (s *stubLabelRepo) GetLabelByID(id, projectID uuid.UUID) (*models.Label, *response.Error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	l, ok := s.labels[id]
	if !ok || l.ProjectID != projectID {
		return nil, &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Label not found"}
	}
	return l, nil
}

func (s *stubLabelRepo) UpdateLabel(label *models.Label) *response.Error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.labels[label.ID] = label
	return nil
}

func (s *stubLabelRepo) DeleteLabel(id, projectID uuid.UUID) *response.Error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.labels[id]; !ok {
		return &response.Error{Code: response.ErrNotFound, StatusCode: 404, Message: "Label not found"}
	}
	delete(s.labels, id)
	return nil
}

func (s *stubLabelRepo) GetLabelsByProjectID(projectID uuid.UUID) ([]models.Label, *response.Error) {
	var res []models.Label
	for _, l := range s.labels {
		if l.ProjectID == projectID {
			res = append(res, *l)
		}
	}
	return res, nil
}

func (s *stubLabelRepo) IsLabelNameExists(projectID uuid.UUID, name string) (bool, *response.Error) {
	for _, l := range s.labels {
		if l.ProjectID == projectID && l.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func TestLabelService_CreateLabel(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)}}
	projectRepo := &stubProjectRepo{
		project:     models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember:    true,
		projectRole: string(dto.ProjectRoleProjectManager),
	}
	labelRepo := &stubLabelRepo{labels: make(map[uuid.UUID]*models.Label)}
	service := services.InitLabelService(labelRepo, projectRepo, authRepo, zap.NewNop())

	// 1. Success Create Label
	req := dto.CreateLabelRequest{
		Name:           "Bug",
		Color:          "#FF0000",
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	}
	res, err := service.CreateLabel(req)
	if err != nil {
		t.Fatalf("expected create label to succeed, got: %v", err)
	}
	if res.Name != "bug" || res.Color != "#FF0000" {
		t.Errorf("created label has unexpected values: %+v", res)
	}

	// 2. Duplicate Check
	_, err = service.CreateLabel(req)
	if err == nil || err.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate label name, got: %v", err)
	}

	// 3. Validation - Name Length too long
	req.Name = "AVeryLongLabelNameThatExceedsThirtyCharactersLimit"
	_, err = service.CreateLabel(req)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for name > 30 chars, got: %v", err)
	}

	// 4. Validation - Color invalid
	req.Name = "Invalid Color Label"
	req.Color = "not-a-color"
	_, err = service.CreateLabel(req)
	if err == nil || err.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid color format, got: %v", err)
	}

	// 5. Unauthorized - Developer Project Role (cannot manage labels)
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	req.Color = "#00FF00"
	req.Name = "Developer Label"
	_, err = service.CreateLabel(req)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for Developer role, got: %v", err)
	}
}

func TestLabelService_UpdateAndDeleteLabel(t *testing.T) {
	orgID := uuid.Must(uuid.NewV4())
	userID := uuid.Must(uuid.NewV4())
	projectID := uuid.Must(uuid.NewV4())
	labelID := uuid.Must(uuid.NewV4())

	authRepo := &sprintAuthRepoStub{user: models.User{ID: userID, OrganizationID: &orgID, Role: string(dto.RoleMember)}}
	projectRepo := &stubProjectRepo{
		project:     models.Project{ID: projectID, OrganizationID: orgID, Name: "Project A"},
		isMember:    true,
		projectRole: string(dto.ProjectRoleProjectManager),
	}
	labelRepo := &stubLabelRepo{labels: map[uuid.UUID]*models.Label{
		labelID: {
			ID:        labelID,
			ProjectID: projectID,
			Name:      "Old Name",
			Color:     "#000000",
		},
	}}
	service := services.InitLabelService(labelRepo, projectRepo, authRepo, zap.NewNop())

	// 1. Success Update Label
	newName := "New Name"
	newColor := "#FFFFFF"
	updateReq := dto.UpdateLabelRequest{
		Name:           &newName,
		Color:          &newColor,
		LabelID:        labelID,
		ProjectID:      projectID,
		UserID:         userID,
		OrganizationID: orgID,
	}

	res, err := service.UpdateLabel(updateReq)
	if err != nil {
		t.Fatalf("expected update to succeed, got: %v", err)
	}
	if res.Name != "new name" || res.Color != "#FFFFFF" {
		t.Errorf("updated label has unexpected values: %+v", res)
	}

	// 2. Unauthorized Delete
	projectRepo.projectRole = string(dto.ProjectRoleDeveloper)
	err = service.DeleteLabel(labelID, projectID, userID, orgID)
	if err == nil || err.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for delete label by developer, got: %v", err)
	}

	// 3. Success Delete
	projectRepo.projectRole = string(dto.ProjectRoleProjectManager)
	err = service.DeleteLabel(labelID, projectID, userID, orgID)
	if err != nil {
		t.Fatalf("expected delete to succeed, got: %v", err)
	}

	// check deleted
	_, err = labelRepo.GetLabelByID(labelID, projectID)
	if err == nil || err.StatusCode != http.StatusNotFound {
		t.Errorf("expected label to be deleted from repo")
	}
}
