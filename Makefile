.PHONY: build run run-worker dev test lint migrate docker-up docker-down tidy

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

run:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

dev: docker-up run

test:
	go test ./... -v

lint:
	go vet ./...

migrate:
	@echo "Run migrations against DATABASE_URL"
	@for f in migrations/*.sql; do echo "Applying $$f"; psql "$$DATABASE_URL" -f "$$f"; done

docker-up:
	docker compose up -d

docker-down:
	docker compose down

tidy:
	go mod tidy
