# =============================================================================
# 应用元信息
#   APP_NAME 用来给二进制和帮助文案命名。
#   VERSION 默认从 git tag 推导；release 构建会通过 -ldflags 注入到 main.version。
# =============================================================================
APP_NAME := assassin-android-controller
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# =============================================================================
# 工具命令
#   注意：这里**不要**重新定义 MAKE。GNU make 内置的 $(MAKE) 会自动传递 jobserver
#   等参数，自己覆盖会破坏并行构建。
# =============================================================================
GO       := go
GOFMT    := gofmt
GOIMPORTS:= goimports
PNPM     := corepack pnpm

# =============================================================================
# 路径
# =============================================================================
CMD_DIR        := ./cmd/server
WEB_DIR        := ./web
BIN_DIR        := ./bin
BIN            := $(BIN_DIR)/$(APP_NAME)
WEB_DIST       := $(WEB_DIR)/dist
EMBED_DIST     := $(CMD_DIR)/dist
COVERAGE_FILE  := coverage.out
COVERAGE_HTML  := coverage.html

# =============================================================================
# 构建参数
#   -trimpath 去掉本机绝对路径，让多人构建出的二进制可复现。
#   -ldflags "-s -w" 去掉调试信息缩小体积。
#   -X main.version=... 把版本号烙进 main 包。
# =============================================================================
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath -ldflags '$(LDFLAGS)'

# =============================================================================
# 默认目标设为 help：直接执行 `make` 会列出所有可用命令，而不是不小心安装依赖。
# =============================================================================
.DEFAULT_GOAL := help

.PHONY: all install build build-backend build-frontend run \
        dev dev-backend dev-frontend \
        test test-backend test-frontend test-e2e coverage \
        fmt fmt-backend fmt-frontend \
        lint lint-backend lint-frontend \
        generate gen-types tidy clean help

## all: 等价于 build，给习惯用 `make all` 的同学留个别名。
all: build

## install: 安装前后端依赖
install:
	$(GO) mod download
	cd $(WEB_DIR) && $(PNPM) install

## build: 构建后端二进制 + 前端静态资源
build: build-frontend build-backend

## build-backend: 编译后端二进制（自动把 web/dist 拷到 cmd/server/dist 供 go:embed 使用）
##
## go:embed 的目标必须在源码同目录树内，所以 build 时把 web/dist 临时拷到 cmd/server/dist。
## 编译结束后通过 trap 还原成只剩 .gitkeep 的占位状态：
##   - 防止后续 `go run` / 单元测试把上一次 build 的旧 dist 也嵌进去；
##   - 也避免本地 git 出现一堆未跟踪的构建产物。
build-backend: build-frontend
	@mkdir -p $(BIN_DIR)
	@trap 'rm -rf $(EMBED_DIST); mkdir -p $(EMBED_DIST); touch $(EMBED_DIST)/.gitkeep' EXIT; \
	rm -rf $(EMBED_DIST); mkdir -p $(EMBED_DIST); \
	cp -R $(WEB_DIST)/. $(EMBED_DIST)/; \
	$(GO) build $(GOFLAGS) -o $(BIN) $(CMD_DIR)

## build-frontend: 构建前端静态资源
build-frontend:
	cd $(WEB_DIR) && $(PNPM) build

## run: 编译并运行后端
run: build
	$(BIN)

## dev: 并行启动后端和前端开发服务器（任意一边退出就一起退出）
##
## 注意：不要写 `trap 'kill 0' EXIT INT TERM` —— `kill 0` 会把信号发给
## 包含 trap shell 自己的整个进程组，bash 5.3 在 trap 递归路径里有 bug
## (run_pending_traps → parse_and_execute → run_pending_traps …) 会爆栈
## SIGSEGV。这里改成：trap 时先清自己的 trap，再杀子进程，保证只触发一次。
dev:
	@trap 'trap - EXIT INT TERM; kill $$BACK_PID $$FRONT_PID 2>/dev/null; wait' EXIT INT TERM; \
	$(MAKE) --no-print-directory dev-backend & BACK_PID=$$!; \
	$(MAKE) --no-print-directory dev-frontend & FRONT_PID=$$!; \
	wait -n; \
	kill $$BACK_PID $$FRONT_PID 2>/dev/null; \
	wait

## dev-backend: 启动后端开发服务（默认读取 configs/config.dev.yaml）
##
## 不再依赖 build tag：现在只有一份二进制，通过 -config 选择具体的配置文件。
## 想跑生产配置就 `APP_CONFIG=configs/config.prod.yaml make dev-backend` 或自行传 -config。
dev-backend:
	$(GO) run $(CMD_DIR)

## dev-frontend: 启动前端开发服务器
dev-frontend:
	cd $(WEB_DIR) && $(PNPM) dev --host 127.0.0.1 --port 5173

## test: 运行所有测试
test: test-backend test-frontend

## test-backend: 运行后端 Go 测试（开启 race 检测器）
test-backend:
	$(GO) test -race ./...

## test-frontend: 运行前端单元测试
test-frontend:
	cd $(WEB_DIR) && $(PNPM) test

## test-e2e: 运行 Playwright E2E（需要前后端已启动；详见 web/playwright.config.ts）
test-e2e:
	cd $(WEB_DIR) && $(PNPM) test:e2e

## coverage: 生成后端覆盖率报告，输出到 coverage.html
coverage:
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) ./...
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "coverage report: $(COVERAGE_HTML)"

## fmt: 格式化所有代码
fmt: fmt-backend fmt-frontend

## fmt-backend: 格式化 Go 代码
fmt-backend:
	$(GOIMPORTS) -w -local assassin-android-controller ./cmd ./internal ./pkg; \

## fmt-frontend: 格式化前端代码
fmt-frontend:
	cd $(WEB_DIR) && $(PNPM) run format

## lint: 运行所有代码检查
lint: lint-backend lint-frontend

## lint-backend: 运行 Go 静态分析（依赖 golangci-lint v2）
lint-backend:
	golangci-lint run ./...

## lint-frontend: 运行前端代码检查
lint-frontend:
	cd $(WEB_DIR) && $(PNPM) run lint

## generate: 运行代码生成
generate:
	$(GO) generate ./...

## gen-types: 用 tygo 把 Go DTO 生成到 web/src/generated/types.ts
##
## 需要先 `go install github.com/gzuidhof/tygo@latest`，CI 不可用时回退到手写镜像。
gen-types:
	tygo generate

## tidy: 整理 go 模块依赖
tidy:
	$(GO) mod tidy

## clean: 清理构建产物
clean:
	rm -rf $(BIN_DIR) $(WEB_DIST) $(COVERAGE_FILE) $(COVERAGE_HTML) $(EMBED_DIST)

## help: 显示帮助信息
help:
	@grep '^##' $(MAKEFILE_LIST) | sed 's/^## //'
