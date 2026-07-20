# ms-kanban-server

A multi-tenant project and task management backend API, built with Go, PostgreSQL, and Redis.

## Tech Stack

- **Backend**: Go 1.26+
- **Web Framework**: Gin
- **Database**: PostgreSQL 15+
- **ORM**: GORM
- **Cache**: Redis 7+
- **Auth**: JWT (access + refresh tokens)
- **API Documentation**: Swagger/OpenAPI (swaggo)
- **Containerization**: Docker + Docker Compose

## Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Redis 7+
- Docker (optional, for Docker Compose setup)

### Setup (Docker Compose)

```bash
# Clone the repo
git clone <repo-url>
cd ms-kanban-server

# Copy env example
cp env.example.sh .env

# Start services
docker-compose up -d

# Run migrations
# (see cmd/api/main.go for migration logic)
go run cmd/api/main.go migrate

# Start the server
go run cmd/api/main.go
```

### Local Development

```bash
# Install dependencies
go mod download

# Run the server
go run cmd/api/main.go
```

## API Documentation

API documentation is available via Swagger UI at `/swagger/index.html` when the server is running.

## Features

- **Authentication & Authorization (JWT + RBAC)**
- Organization & User Management
- Project Management
- Sprint Management
- Task Management
- Comments & Mentions
- Labels
- Attachments
- Notifications
- Activity Logs
- Dashboards
- Reports
- Search & Filtering

## Project Structure

```
ms-kanban-server/
├── cmd/api/           # Main entry point
├── config/            # Configuration loading
├── docs/              # Swagger docs
├── drivers/           # DB & Redis drivers & migration
├── internal/
│   ├── handlers/     # HTTP handlers & DTOs
│   ├── middleware/ # Middleware (auth, RBAC)
│   ├── pkg/        # Utilities, email, logger, models, responses
│   ├── repository/ # Data access layer
│   ├── routes/     # Route setup
│   └── services/   # Business logic
├── migrations/       # Database migrations
└── templates/      # Email templates
```
