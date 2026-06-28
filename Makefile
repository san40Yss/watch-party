.PHONY: up down logs build dev deps

# Pull deps and build image, then start all services
up:
	docker compose up --build -d

# Stop and remove containers (keeps volumes / DB data)
down:
	docker compose down

# Destroy everything including DB volume
clean:
	docker compose down -v

logs:
	docker compose logs -f

build:
	docker compose build

# Download Go deps locally (needed for local dev without Docker)
deps:
	go mod tidy

# Run the Go server locally (requires Postgres running and .env sourced)
dev:
	go run ./cmd/server
