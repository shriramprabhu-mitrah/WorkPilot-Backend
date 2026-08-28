package handlers

import "github.com/ms-kanban-server/api/services"

func InitHandler(service services.Service) *handler {
	return &handler{
		service: service,
	}
}

type handler struct {
	service services.Service
}
