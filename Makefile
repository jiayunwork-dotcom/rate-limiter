.PHONY: all build build-gateway build-admin build-frontend test lint clean \
        docker-up docker-down docker-logs db-reset redis-reset \
        dev-gateway dev-admin dev-frontend format help

PROJECT_NAME := rate-limiter
GOPROXY := https://goproxy.cn,direct
GOPATH ?= $(HOME)/go

help:
	@echo "速率限制网关平台 Makefile"
	@echo "=========================="
	@echo ""
	@echo "构建命令:"
	@echo "  make build              构建所有Go服务"
	@echo "  make build-gateway      构建网关服务"
	@echo "  make build-admin        构建管理API服务"
	@echo "  make build-frontend     构建前端(需要Node.js)"
	@echo ""
	@echo "开发命令:"
	@echo "  make dev-gateway        运行网关服务(热更新需要air)"
	@echo "  make dev-admin          运行管理API服务(热更新需要air)"
	@echo "  make dev-frontend       运行前端开发服务器"
	@echo "  make format             格式化Go代码"
	@echo "  make lint               运行Go静态检查"
	@echo "  make test               运行Go单元测试"
	@echo ""
	@echo "Docker命令:"
	@echo "  make docker-up          启动所有容器"
	@echo "  make docker-down        停止并移除所有容器"
	@echo "  make docker-logs        查看所有服务日志"
	@echo "  make docker-build       重新构建所有容器镜像"
	@echo "  make db-reset           重置PostgreSQL数据"
	@echo "  make redis-reset        重置Redis数据"
	@echo ""
	@echo "清理命令:"
	@echo "  make clean              清理构建产物和临时文件"

all: build

build: build-gateway build-admin

build-gateway:
	@echo "==> 构建网关服务..."
	@cd gateway && GOPROXY=$(GOPROXY) CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/gateway ./cmd
	@echo "  ✓ 网关服务构建完成: bin/gateway"

build-admin:
	@echo "==> 构建管理API服务..."
	@cd admin-api && GOPROXY=$(GOPROXY) CGO_ENABLED=0 go build -ldflags="-w -s" -o ../bin/admin-api ./cmd
	@echo "  ✓ 管理API构建完成: bin/admin-api"

build-frontend:
	@echo "==> 构建前端..."
	@cd admin-frontend && npm install && npm run build:prod
	@echo "  ✓ 前端构建完成: admin-frontend/dist/"

test:
	@echo "==> 运行网关单元测试..."
	@cd gateway && GOPROXY=$(GOPROXY) go test -v -race -coverprofile=../coverage-gateway.out ./...
	@echo "==> 运行管理API单元测试..."
	@cd admin-api && GOPROXY=$(GOPROXY) go test -v -race -coverprofile=../coverage-admin.out ./...

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "安装 golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.55.2; \
	fi
	@echo "==> 检查网关代码..."
	@cd gateway && golangci-lint run ./...
	@echo "==> 检查管理API代码..."
	@cd admin-api && golangci-lint run ./...

format:
	@echo "==> 格式化Go代码..."
	@cd gateway && gofmt -w -s .
	@cd admin-api && gofmt -w -s .
	@echo "  ✓ 代码格式化完成"

dev-gateway:
	@echo "==> 启动网关服务 (开发模式)..."
	@cd gateway && GOPROXY=$(GOPROXY) go run ./cmd

dev-admin:
	@echo "==> 启动管理API服务 (开发模式)..."
	@cd admin-api && GOPROXY=$(GOPROXY) go run ./cmd

dev-frontend:
	@echo "==> 启动前端开发服务器..."
	@cd admin-frontend && npm start

docker-up:
	@echo "==> 启动所有服务..."
	docker compose up -d --build
	@echo ""
	@echo "  ✓ 服务启动完成:"
	@echo "    - 前端管理界面: http://localhost:8090"
	@echo "    - 管理API:      http://localhost:8081/api/v1/health"
	@echo "    - 网关HTTP:     http://localhost:8180"
	@echo "    - 网关gRPC:     localhost:9190"
	@echo "    - 网关指标:     http://localhost:8180/metrics (ratelimiter_*自定义限流指标)"
	@echo "    - 管理指标:     http://localhost:8090/metrics/admin"
	@echo "    - PostgreSQL:   localhost:5432"
	@echo "    - Redis:        localhost:6379"
	@echo ""
	@echo "  ⚠️  初次启动需要等待PostgreSQL和Redis健康检查通过"

docker-down:
	@echo "==> 停止所有服务..."
	docker compose down
	@echo "  ✓ 所有服务已停止"

docker-logs:
	docker compose logs -f

docker-build:
	@echo "==> 重新构建所有容器镜像..."
	docker compose build --no-cache
	@echo "  ✓ 镜像构建完成"

db-reset:
	@echo "==> 重置PostgreSQL数据..."
	docker compose down postgres
	docker volume rm $(PROJECT_NAME)-postgres-data 2>/dev/null || true
	@echo "  ✓ PostgreSQL数据已重置，下次启动会重新初始化"

redis-reset:
	@echo "==> 重置Redis数据..."
	docker compose down redis
	docker volume rm $(PROJECT_NAME)-redis-data 2>/dev/null || true
	@echo "  ✓ Redis数据已重置"

clean:
	@echo "==> 清理构建产物..."
	@rm -rf bin/
	@rm -rf coverage-*.out
	@cd admin-frontend && rm -rf dist/ node_modules/ 2>/dev/null || true
	@go clean -cache -testcache 2>/dev/null || true
	@echo "  ✓ 清理完成"
