# WorkPilot (Backend)

## Overview

**WorkPilot (Backend)** is the backend API for the **TaskFlow** platform, a multi‑tenant project and task management system. It is written in **Go** using **Clean Architecture** (Handler → Service → Repository → Domain) and exposes a **RESTful** API consumed by web and mobile front‑ends.

The repository implements the functional specifications described in the **TaskFlow SRS & Agile Backlog** document ([TaskFlow SRS and Agile Backlog](/C:/Users/Mitrah/Downloads/TaskFlow_SRS_and_Agile_Backlog.md)).

---

## Features

- **Authentication & Authorization** – JWT access & refresh tokens, RBAC middleware, password reset, OTP (mock).
- **Organization Management** – Create, update, delete, and retrieve organization details.
- **Project Management** – Create, update, delete, and retrieve projects within an organization.
- **Public Endpoints** – Health check and country lookup (cached).

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | **Go 1.26+** |
| Web Framework | **Gin** |
| Database | **PostgreSQL** (via GORM) |
| Cache | **Redis** |
| Auth | **JWT** (access & refresh) |
| Password Hashing | **bcrypt** |
| Logging | **Uber Zap** |
| Config | **environment variables** |
| Validation | **go-playground/validator v10** |
| Email | **Brevo API (Primary) & Resend API (Fallback)** |
| Testing | **testing + testify** |
| Containerization | **Docker & Docker‑Compose** |
| CI/CD | **GitHub Actions** |
| Architecture | **Clean Architecture** |

---

## Getting Started

### Prerequisites

- Go 1.26+ installed
- PostgreSQL 15+ running
- Redis 7+ running
- Docker & Docker‑Compose (optional, for containerized setup)

### Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/ms-kanban-server.git
   cd ms-kanban-server
   ```
2. Copy the example environment file and configure your credentials:
   ```bash
   cp env.example.sh .env
   # edit .env (DB connection, Redis, JWT secret, etc.)
   ```
3. **Run locally** (without Docker):
   ```bash
   go run cmd/api/main.go
   ```
   The server starts on the port defined in `.env` (default `6369`).
4. **Run with Docker Compose** (includes PostgreSQL and Redis):
   ```bash
   docker-compose up --build
   ```
   The API will be reachable at `http://localhost:6369/api/v1`.

---

## Running Tests

```bash
go test ./... -cover
```
All unit and integration tests are located under the `internal/...` packages.

---

## API Documentation

Swagger UI is generated automatically and served at:
```
GET api/v1/swagger/index.html
```
The OpenAPI JSON can be accessed at `/swagger/doc.json`.

---

## Project Structure (Clean Architecture)

```
internal/
├── handlers/      # HTTP controllers (Gin handlers)
│   └── http/      # auth, organization, project, public handlers
├── middleware/    # Auth, RBAC, logging, rate‑limit
├── models/        # DTOs, request/response structs
├── pkg/           # Utilities (logger, email, etc.)
├── repository/    # Data access layer (GORM, Redis)
├── services/      # Business logic and orchestration
├── routes/        # Route definitions
└── ...
```
The `cmd/api/main.go` file bootstraps the application, loads configuration, initializes the logger, database, Redis client, and registers routes.

---
