package favoriterepo

import (
	"errors"
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"github.com/ms-kanban-server/internal/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *favoriteDatabase) AddFavorite(favorite *models.Favorite) *response.Error {
	if err := d.db.Create(favorite).Error; err != nil {
		if utils.IsDuplicateKeyError(err) {
			d.logger.Error("Duplicate Key Error", zap.Error(err))
			return &response.Error{
				Code:       response.ErrConflict,
				StatusCode: http.StatusConflict,
				Message:    "Item is already added to favorites",
			}
		}
		d.logger.Error("Failed to add favorite", zap.Error(err))
		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to add item to favorites",
		}
	}
	return nil
}

func (d *favoriteDatabase) RemoveFavorite(userID uuid.UUID, itemType string, itemID uuid.UUID) (*models.Favorite, *response.Error) {
	fav, err := d.GetFavoriteByUserAndItem(userID, itemType, itemID)
	if err != nil {
		return nil, err
	}

	result := d.db.Delete(fav)
	if result.Error != nil {
		d.logger.Error("Failed to remove favorite", zap.Error(result.Error))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to remove item from favorites",
		}
	}

	return fav, nil
}

func (d *favoriteDatabase) RemoveFavoriteByID(userID uuid.UUID, favoriteID uuid.UUID) (*models.Favorite, *response.Error) {
	var fav models.Favorite
	err := d.db.Where("id = ? AND user_id = ?", favoriteID, userID).First(&fav).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Favorite record not found",
			}
		}
		d.logger.Error("Failed to get favorite by ID", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to remove favorite",
		}
	}

	if err := d.db.Delete(&fav).Error; err != nil {
		d.logger.Error("Failed to remove favorite by ID", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to remove favorite",
		}
	}

	return &fav, nil
}

func (d *favoriteDatabase) GetFavoritesByUserID(userID uuid.UUID, itemType string) ([]models.Favorite, *response.Error) {
	var favorites []models.Favorite
	query := d.db.Where("user_id = ?", userID)

	if itemType != "" {
		query = query.Where("item_type = ?", itemType)
	}

	err := query.
		Preload("UserStory").
		Preload("UserStory.Project").
		Preload("UserStory.Assignee").
		Preload("UserStory.Reporter").
		Preload("Task").
		Preload("Task.Project").
		Preload("Task.Assignee").
		Preload("Task.Reporter").
		Preload("Task.Labels").
		Order("created_at DESC").
		Find(&favorites).Error

	if err != nil {
		d.logger.Error("Failed to fetch favorites", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch favorites",
		}
	}

	return favorites, nil
}

func (d *favoriteDatabase) GetFavoriteByUserAndItem(userID uuid.UUID, itemType string, itemID uuid.UUID) (*models.Favorite, *response.Error) {
	var favorite models.Favorite
	query := d.db.Where("user_id = ? AND item_type = ?", userID, itemType)

	switch itemType {
	case models.FavoriteItemTypeUserStory:
		query = query.Where("user_story_id = ?", itemID)
	case models.FavoriteItemTypeTask:
		query = query.Where("task_id = ?", itemID)
	default:
		return nil, &response.Error{
			Code:       response.ErrValidation,
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid item type",
		}
	}

	err := query.First(&favorite).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Favorite record not found",
			}
		}
		d.logger.Error("Failed to get favorite", zap.Error(err))
		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to get favorite",
		}
	}

	return &favorite, nil
}

func (d *favoriteDatabase) IsFavorited(userID uuid.UUID, itemType string, itemID uuid.UUID) (bool, *response.Error) {
	var count int64
	query := d.db.Model(&models.Favorite{}).Where("user_id = ? AND item_type = ?", userID, itemType)

	switch itemType {
	case models.FavoriteItemTypeUserStory:
		query = query.Where("user_story_id = ?", itemID)
	case models.FavoriteItemTypeTask:
		query = query.Where("task_id = ?", itemID)
	default:
		return false, nil
	}

	if err := query.Count(&count).Error; err != nil {
		d.logger.Error("Failed to check if item is favorited", zap.Error(err))
		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to check favorite status",
		}
	}

	return count > 0, nil
}
