package commentsrepo

import (
	"math"
	"net/http"
	"time"

	"github.com/gofrs/uuid"
	requestdto "github.com/ms-kanban-server/internal/handlers/dto/request"
	"github.com/ms-kanban-server/internal/pkg/models"
	"github.com/ms-kanban-server/internal/pkg/response"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (d *commentsDatabase) CreateComment(comment models.Comments) *response.Error {

	if err := d.db.Create(&comment).Error; err != nil {

		d.logger.Error("Failed to create comment",
			zap.Error(err),
			zap.String("TaskID", comment.TaskID.String()),
			zap.String("UserID", comment.UserID.String()))

		return &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to create comment",
		}
	}

	return nil
}

func (d *commentsDatabase) GetCommentByID(commentID uuid.UUID) (*models.Comments, *response.Error) {

	var comment models.Comments

	if err := d.db.
		Preload("User").
		Preload("ParentComment").
		Preload("ParentComment.User").
		Preload("Attachments").
		First(&comment, "id = ?", commentID).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			return nil, &response.Error{
				Code:       response.ErrNotFound,
				StatusCode: http.StatusNotFound,
				Message:    "Comment not found",
			}
		}

		d.logger.Error("Failed to get comment",
			zap.Error(err),
			zap.String("CommentID", commentID.String()))

		return nil, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Failed to fetch comment",
		}
	}

	return &comment, nil
}

func (d *commentsDatabase) GetCommentsByTaskID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {

	var (
		comments   []models.Comments
		totalItems int64
	)

	req.PaginationQuery.Normalize(10)

	offset := (req.Page - 1) * req.PageSize

	baseQuery := d.db.Model(&models.Comments{}).
		Where("organization_id = ? AND task_id = ? AND parent_comment_id IS NULL", req.OrganizationID, req.TaskID)

	if err := baseQuery.Count(&totalItems).Error; err != nil {

		d.logger.Error("Database error occurred",
			zap.String("TaskID", req.TaskID.String()),
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("User").
		Preload("ParentComment").
		Preload("ParentComment.User").
		Preload("Attachments").
		Order("created_at DESC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&comments).Error; err != nil {

		d.logger.Error("Failed to fetch task comments",
			zap.String("TaskID", req.TaskID.String()),
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(req.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        req.Page,
		PageSize:    req.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     req.Page < totalPages,
		HasPrevious: req.Page > 1,
	}

	return comments, pagination, nil
}

func (d *commentsDatabase) UpdateComment(commentID uuid.UUID, req models.Comments) *response.Error {

	result := d.db.
		Model(&models.Comments{}).
		Where("id = ?", commentID).
		Updates(req)

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

		d.logger.Error("CommentID not found",
			zap.String("comment_id", commentID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Comment not found",
		}
	}

	return nil
}

func (d *commentsDatabase) DeleteComment(commentID uuid.UUID) *response.Error {

	result := d.db.Model(&models.Comments{}).
		Where("id = ?", commentID).
		Updates(map[string]interface{}{
			"is_deleted": true,
			"deleted_at": time.Now(),
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

		d.logger.Error("CommentID not found",
			zap.String("comment_id", commentID.String()))

		return &response.Error{
			Code:       response.ErrNotFound,
			StatusCode: http.StatusNotFound,
			Message:    "Comment not found",
		}
	}

	return nil
}

func (d *commentsDatabase) GetCommentsByParentID(req requestdto.GetComments) ([]models.Comments, response.Pagination, *response.Error) {

	var (
		comments   []models.Comments
		totalItems int64
	)

	req.PaginationQuery.Normalize(10)

	offset := (req.Page - 1) * req.PageSize

	baseQuery := d.db.Model(&models.Comments{}).
		Where("organization_id = ? AND task_id = ? AND parent_comment_id = ?", req.OrganizationID, req.TaskID, req.CommentID)

	if err := baseQuery.Count(&totalItems).Error; err != nil {

		d.logger.Error("Database error occurred",
			zap.String("TaskID", req.TaskID.String()),
			zap.String("ParentCommentID", req.CommentID.String()),
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	if err := baseQuery.
		Preload("User").
		Preload("ParentComment").
		Preload("ParentComment.User").
		Preload("Attachments").
		Order("created_at ASC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&comments).Error; err != nil {

		d.logger.Error("Failed to fetch reply comments",
			zap.String("TaskID", req.TaskID.String()),
			zap.String("ParentCommentID", req.CommentID.String()),
			zap.Error(err))

		return nil, response.Pagination{}, &response.Error{
			Code:       response.ErrInternalServerError,
			StatusCode: http.StatusInternalServerError,
			Message:    "Something went wrong. Please try again later.",
		}
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(req.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	pagination := response.Pagination{
		Page:        req.Page,
		PageSize:    req.PageSize,
		TotalItems:  int(totalItems),
		TotalPages:  totalPages,
		HasNext:     req.Page < totalPages,
		HasPrevious: req.Page > 1,
	}

	return comments, pagination, nil
}
