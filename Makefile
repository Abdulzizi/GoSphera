.PHONY: dev-backend dev-frontend build-backend build-frontend test lint docker-up docker-down

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

build-backend:
	cd backend && go build -o bin/server ./cmd/server

build-frontend:
	cd frontend && npm run build

test:
	cd backend && go test -v ./pkg/...

lint:
	cd backend && go vet ./...
	cd frontend && npm run type-check

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down --remove-orphans
