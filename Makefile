.PHONY: help backend-test frontend-install frontend-build test docker-build docker-up docker-down deploy release

REGISTRY      ?= docker.io
IMAGE         ?= ljtian/trendinghub
TAG           ?= local
BASE_REGISTRY ?= docker.io
RELEASE_DATE  ?= $(shell date +%Y%m%d)
RELEASE_TAG   ?= release-$(RELEASE_DATE)

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
	@echo "  release          Create and push git tag release-YYYYMMDD"

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

release:
	@git diff --quiet || (echo "Working tree not clean; commit or stash changes before releasing."; exit 1)
	@git tag $(RELEASE_TAG)
	@git push origin $(RELEASE_TAG)
	@echo "Pushed tag $(RELEASE_TAG). GitHub Actions release workflow will build and push the image."
