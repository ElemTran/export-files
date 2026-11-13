# =============================================================================
# export-files —— 导出目录/文件内容为 Markdown
# 门禁目标与 star-entry-gateway/cloudflare 保持一致：fmt / fix / tidy / lint /
# test / deadcode / vuln，全量检查入口为 `make check`。
# =============================================================================

GO ?= go
GOFMT ?= gofmt
GOLANGCI_LINT ?= golangci-lint

MODULE_PKGS ?= ./...
ENTRY_PKGS ?= ./cmd/...
FMT_PATHS ?= cmd internal

BIN_DIR ?= dist
BUILD_FLAGS ?= -trimpath
LDFLAGS ?= -s -w

.PHONY: all build build-linux build-windows clean \
	fmt fmt-check fix fix-check tidy tidy-check \
	lint vuln deadcode test check pre-commit precommit \
	install-lint install-tools

all: check build

# ---------------------------------------------------------------------------
# 格式化与依赖整理
# ---------------------------------------------------------------------------

fmt: ## 格式化 Go 源码
	$(GOFMT) -w $(FMT_PATHS)

fmt-check: ## 检查源码格式
	@out="$$($(GOFMT) -l $(FMT_PATHS))"; \
	if [ -n "$$out" ]; then \
		echo "以下文件未 gofmt:"; \
		echo "$$out"; \
		exit 1; \
	fi

fix: fmt ## 运行 go fix 与 modernize 自动修复
	@echo "--> 正在运行官方 go fix..."
	@$(GO) fix $(MODULE_PKGS)
	@echo "--> 正在运行现代语法重构 (modernize)..."
	@$(GO) tool modernize -fix $(MODULE_PKGS)
	@echo "--> 格式化与重构完成！"

fix-check: ## 检查是否有未通过 go fix 或 modernize 修复的代码
	@before="$$(git status --porcelain)"; \
	$(GO) fix $(MODULE_PKGS); \
	$(GO) tool modernize -fix $(MODULE_PKGS); \
	after="$$(git status --porcelain)"; \
	if [ "$$before" != "$$after" ]; then \
		echo "错误: 存在未通过 go fix 或 modernize 修复的代码，请在本地运行 make fix 并提交:"; \
		git diff; \
		exit 1; \
	fi

tidy: ## 整理并清理未使用的依赖
	$(GO) mod tidy

tidy-check: ## 检查 go.mod 与 go.sum 是否保持 clean/tidy
	@before_mod="$$(git status --porcelain go.mod go.sum)"; \
	$(GO) mod tidy; \
	after_mod="$$(git status --porcelain go.mod go.sum)"; \
	if [ "$$before_mod" != "$$after_mod" ]; then \
		echo "错误: go.mod 或 go.sum 存在未清理的依赖，请在本地运行 go mod tidy 并提交:"; \
		git diff go.mod go.sum; \
		exit 1; \
	fi

# ---------------------------------------------------------------------------
# 静态检查与测试
# ---------------------------------------------------------------------------

lint: ## 运行静态检查 (golangci-lint)
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "未找到 $(GOLANGCI_LINT)。请执行: make install-lint"; \
		echo "或: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
		exit 1; \
	}
	$(GOLANGCI_LINT) run --timeout 2m $(MODULE_PKGS)

vuln: ## 运行 Go 官方漏洞扫描 (govulncheck)
	$(GO) tool govulncheck $(MODULE_PKGS)

deadcode: ## 运行 Go 官方死代码扫描 (deadcode)
	$(GO) tool deadcode $(ENTRY_PKGS)

test: ## 运行 Go 单元测试
	CGO_ENABLED=1 $(GO) test -race $(MODULE_PKGS) -count=1

check: fmt-check fix-check tidy-check lint test deadcode vuln ## 运行全量代码与安全门禁
	@echo "check 全部通过"

pre-commit: check ## 提交前门禁检查
precommit: pre-commit

# ---------------------------------------------------------------------------
# 构建
# ---------------------------------------------------------------------------

build: build-linux build-windows ## 构建 linux/amd64 与 windows/amd64 产物

build-linux: ## 构建 linux/amd64 产物
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/export-files-linux-amd64 ./cmd/export-files

build-windows: ## 构建 windows/amd64 产物
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/export-files-windows-amd64.exe ./cmd/export-files

clean: ## 清理构建产物
	rm -rf $(BIN_DIR)

# ---------------------------------------------------------------------------
# 工具链
# ---------------------------------------------------------------------------

install-lint: ## 安装 golangci-lint
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

install-tools: install-lint ## 一键安装开发与检查工具链
	$(GO) mod download
