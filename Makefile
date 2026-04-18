.PHONY: dev dev-web dev-admin dev-api dev-submission build build-web build-admin build-api build-submission proto clean infra infra-down help

help:
	@echo "Available commands:"
	@echo "  make dev            - Start frontend, admin, and all backend services"
	@echo "  make dev-web        - Start only frontend development server"
	@echo "  make dev-admin      - Start only admin development server"
	@echo "  make dev-api        - Start only api backend development server"
	@echo "  make dev-submission - Start only submission backend development server"
	@echo "  make build          - Build frontend, admin, and all backend services"
	@echo "  make build-web      - Build frontend only"
	@echo "  make build-admin    - Build admin only"
	@echo "  make build-api      - Build api backend only"
	@echo "  make build-submission - Build submission backend only"
	@echo "  make proto          - Generate code from protobuf"
	@echo "  make infra          - Start PostgreSQL and RabbitMQ"
	@echo "  make infra-down     - Stop PostgreSQL and RabbitMQ"
	@echo "  make clean          - Clean build artifacts"

dev:
	npm run dev

dev-web:
	npm run dev:web

dev-admin:
	npm run dev:admin

dev-api:
	npm run dev:api

dev-submission:
	npm run dev:submission

build:
	npm run build

build-web:
	npm run build:web

build-admin:
	npm run build:admin

build-api:
	npm run build:api

build-submission:
	npm run build:submission

proto:
	buf generate

clean:
	rm -rf app/web/dist app/admin/dist bin/ services/*/bin/
	cd services/api && go clean -cache
	cd services/submission && go clean -cache

infra:
	docker compose up -d

infra-down:
	docker compose down
