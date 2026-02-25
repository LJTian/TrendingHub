# Stage 1: 构建前端
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: 构建 Go 后端
FROM golang:1.24-alpine AS backend
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /trendinghub ./cmd/api

# Stage 3: 运行镜像
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /trendinghub .
COPY --from=frontend /app/web/dist ./web
ENV APP_PORT=9000 \
    WEB_ROOT=/app/web
EXPOSE 9000
ENTRYPOINT ["./trendinghub"]
