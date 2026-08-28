package services

import (
	"github.com/ms-kanban-server/api/repositories"
)

type Service interface {
}

func InitRepositories(repo repositories.Repository) Service {
	return &service{
		Repo: repo,
	}
}

type service struct {
	Repo repositories.Repository
}
