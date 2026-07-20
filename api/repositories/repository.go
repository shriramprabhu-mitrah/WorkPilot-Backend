package repositories

import "gorm.io/gorm"

type Repository interface {
}

func InitRepositories(db *gorm.DB) Repository {
	return &repository{
		DB: db,
	}
}

type repository struct {
	DB *gorm.DB
}
