.PHONY: help backend-test frontend-install frontend-build test docker-build docker-up docker-down deploy

REGISTRY      ?= docker.io
IMAGE         ?= ljtian/trendinghub
TAG           ?= local
BASE_REGISTRY ?= docker.io

help:
	@echo "Targets:"
	@echo "  backend-test     Run Go unit tests (go test ./...)"
	@echo "  frontend-install Install npm deps (npm ci)"
	@echo "  frontend-build   Build Vite frontend (npm run build)"
	@echo "  test             Run backend-test and frontend-build sequentially"
	@echo "  docker-build     Build Docker image (REGISTRY/IMAGE:TAG, BASE_REGISTRY for base images, default docker.io)"
	@echo "  docker-up        docker compose up -d --build"
	@echo "  docker-down      docker compose down"
	@echo "  deploy           Run tests, then docker compose up -d --build"

backend-test:
	go test ./...

frontend-install:
	cd web && npm ci

frontend-build: frontend-install
	cd web && npm run build

test: backend-test frontend-build

docker-build:
	docker build \
		--build-arg BASE_REGISTRY=$(BASE_REGISTRY) \
		-t $(REGISTRY)/$(IMAGE):$(TAG) .

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

deploy: test docker-up
