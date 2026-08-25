package request

import (
	"github.com/gofrs/uuid"
	"github.com/ms-kanban-server/internal/pkg/response"
)

type AddFavoriteRequest struct {
	ItemType       string    `json:"item_type" binding:"required,oneof=user_story task"`
	ItemID         uuid.UUID `json:"item_id" binding:"required"`
	UserID         uuid.UUID `json:"-" swaggerignore:"true"`
	OrganizationID uuid.UUID `json:"-" swaggerignore:"true"`
}

type RemoveFavoriteRequest struct {
	ItemType string    `form:"item_type" json:"item_type" binding:"required,oneof=user_story task"`
	ItemID   uuid.UUID `form:"item_id" json:"item_id" binding:"required"`
}

type GetFavoritesFilter struct {
	response.PaginationQuery
	response.SortQuery
	ItemType string `form:"item_type"`
	Search   string `form:"search"`
}