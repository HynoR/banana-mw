# Makefile for cross-compiling to Linux amd64 on macOS

# 项目名称
APP_NAME := banana-mw
# 输出目录
BUILD_DIR := build
# 输出文件名（均在 build/ 下，不写入项目根目录）
OUTPUT_LINUX := $(BUILD_DIR)/$(APP_NAME)
OUTPUT_LOCAL := $(BUILD_DIR)/$(APP_NAME).local

# 默认目标
.PHONY: all
all: build

# 构建 Linux amd64 生产环境程序
.PHONY: build
build:
	@echo "正在交叉编译 Linux amd64 生产环境程序..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags "-s -w" \
		-o $(OUTPUT_LINUX) ./cmd/banana-mw
	@echo "构建完成: $(OUTPUT_LINUX)"
	@ls -lh $(OUTPUT_LINUX)

# 构建当前系统可执行文件（本地调试）
.PHONY: build-local
build-local:
	@echo "正在编译当前平台程序..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags "-s -w" \
		-o $(OUTPUT_LOCAL) ./cmd/banana-mw
	@echo "构建完成: $(OUTPUT_LOCAL)"
	@ls -lh $(OUTPUT_LOCAL)

# 清理构建文件
.PHONY: clean
clean:
	@echo "清理构建文件..."
	@rm -rf $(BUILD_DIR)
	@echo "清理完成"

# 运行本地测试（macOS）
.PHONY: run
run:
	@echo "运行本地程序..."
	go run ./cmd/banana-mw

# 安装依赖
.PHONY: deps
deps:
	@echo "下载依赖..."
	go mod download
	go mod tidy

# 格式化代码
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 运行测试
.PHONY: test
test:
	@echo "运行测试..."
	go test ./...

# 显示帮助信息
.PHONY: help
help:
	@echo "可用命令:"
	@echo "  make build        - 交叉编译 Linux amd64 到 build/banana-mw"
	@echo "  make build-local  - 编译当前平台到 build/banana-mw.local"
	@echo "  make clean        - 清理 build/ 目录"
	@echo "  make run    - 在本地运行程序（macOS）"
	@echo "  make deps   - 下载并整理依赖"
	@echo "  make fmt    - 格式化代码"
	@echo "  make test   - 运行测试"
	@echo "  make help   - 显示此帮助信息"
