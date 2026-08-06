package commentsrepo

import (
	"net/http"

	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
)

func (d *commentsDatabase) HasReplies(commentID uuid.UUID) (bool, *response.Error) {

	var count int64

	if err := d.db.
		Model(&models.Comments{}).
		Where("parent_comment_id = ? AND deleted_at IS NULL", commentID).
		Count(&count).Error; err != nil {

		d.logger.Error("Failed to check replies",
			zap.Error(err))

		return false, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	return count > 0, nil
}

func (d *commentsDatabase) MarkCommentAsDeleted(commentID uuid.UUID) *response.Error {

	result := d.db.
		Model(&models.Comments{}).
		Where("id = ?", commentID).
		Updates(map[string]interface{}{
			"content": "This comment was deleted.",
		})

	if result.Error != nil {

		d.logger.Error("Database error occurred",
			zap.Error(result.Error))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if result.RowsAffected == 0 {
		d.logger.Error("Comment not found",
			zap.Error(result.Error))
		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Comment not found",
		}
	}

	return nil
}
