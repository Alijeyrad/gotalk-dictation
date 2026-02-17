BINARY    := gotalk-dictation
BUILD_DIR := build
BIN       := $(BUILD_DIR)/$(BINARY)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
PKG       := github.com/Alijeyrad/gotalk-dictation/internal/version
LDFLAGS   := -ldflags="-s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT)"

# --- Colors ---
RESET  := \033[0m
CYAN   := \033[36m
YELLOW := \033[33m
GREEN  := \033[32m
BLUE   := \033[34m
RED    := \033[31m

# --- .PHONY ---
.PHONY: build run clean install uninstall autostart deps tidy fmt vet lint help test test-x11 test-integration test-all

# --- Application --------------------------------------------

build: ## Compile the binary into build/
	@printf "$(BLUE)Building $(BINARY)...$(RESET)\n"
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -trimpath -o $(BIN) .
	@printf "$(GREEN)Built: $(BIN)$(RESET)\n"

run: build ## Build and run (installs .desktop to ~/.local/share/applications/ for portal registration)
	@mkdir -p ~/.local/share/applications
	@install -m644 packaging/com.alijeyrad.GoTalkDictation.desktop \
		~/.local/share/applications/com.alijeyrad.GoTalkDictation.desktop
	@printf "$(BLUE)Starting $(BINARY)...$(RESET)\n"
	@$(BIN)

clean: ## Remove the build directory
	@printf "$(YELLOW)Cleaning...$(RESET)\n"
	@rm -rf $(BUILD_DIR)
	@printf "$(GREEN)Clean complete!$(RESET)\n"

# --- Install ------------------------------------------------

install: build ## Install binary + .desktop file + icon system-wide
	@printf "$(BLUE)Installing $(BINARY)...$(RESET)\n"
	@sudo install -m 755 $(BIN) /usr/local/bin/$(BINARY)
	@sudo install -m 644 packaging/com.alijeyrad.GoTalkDictation.desktop \
		/usr/share/applications/com.alijeyrad.GoTalkDictation.desktop
	@sudo install -m 644 packaging/com.alijeyrad.GoTalkDictation.metainfo.xml \
		/usr/share/metainfo/com.alijeyrad.GoTalkDictation.metainfo.xml 2>/dev/null || true
	@sudo mkdir -p /usr/share/icons/hicolor/128x128/apps
	@sudo install -m 644 internal/ui/assets/icon.png \
		/usr/share/icons/hicolor/128x128/apps/com.alijeyrad.GoTalkDictation.png
	@sudo gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
	@printf "$(GREEN)Installed to /usr/local/bin/$(BINARY)$(RESET)\n"
	@printf "$(GREEN)Desktop entry: /usr/share/applications/com.alijeyrad.GoTalkDictation.desktop$(RESET)\n"

uninstall: ## Remove binary, desktop entry, icon, autostart, and user-local files
	@printf "$(RED)Uninstalling $(BINARY)...$(RESET)\n"
	@pkill -x $(BINARY) 2>/dev/null || true
	@sudo rm -f /usr/local/bin/$(BINARY)
	@sudo rm -f /usr/share/applications/com.alijeyrad.GoTalkDictation.desktop
	@sudo rm -f /usr/share/metainfo/com.alijeyrad.GoTalkDictation.metainfo.xml
	@sudo rm -f /usr/share/icons/hicolor/128x128/apps/com.alijeyrad.GoTalkDictation.png
	@sudo gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
	@sudo update-desktop-database /usr/share/applications 2>/dev/null || true
	@rm -f ~/.local/share/applications/com.alijeyrad.GoTalkDictation.desktop
	@rm -f ~/.config/autostart/$(BINARY).desktop
	@printf "$(GREEN)Uninstalled!$(RESET)\n"
	@printf "$(YELLOW)Note: settings kept at ~/.config/$(BINARY)/ — remove manually if desired$(RESET)\n"

autostart: ## Add a login autostart entry for the current user
	@printf "$(BLUE)Creating autostart entry...$(RESET)\n"
	@mkdir -p ~/.config/autostart
	@BINPATH=$$(command -v $(BINARY) 2>/dev/null || echo "$(CURDIR)/$(BIN)"); \
	printf '[Desktop Entry]\nType=Application\nName=GoTalk Dictation\nExec=%s\nIcon=audio-input-microphone\nComment=System-wide speech-to-text dictation\nCategories=Accessibility;Utility;\nX-GNOME-Autostart-enabled=true\n' \
		"$$BINPATH" > ~/.config/autostart/$(BINARY).desktop
	@printf "$(GREEN)Autostart entry: ~/.config/autostart/$(BINARY).desktop$(RESET)\n"

# --- Development --------------------------------------------

deps: ## Install system build dependencies (X11 headers, GL)
	@printf "$(BLUE)Installing system dependencies...$(RESET)\n"
	@if command -v dnf >/dev/null 2>&1; then \
		sudo dnf install -y libX11-devel libXcursor-devel libXrandr-devel \
			libXinerama-devel libXi-devel libXxf86vm-devel mesa-libGL-devel; \
	elif command -v apt-get >/dev/null 2>&1; then \
		sudo apt-get install -y libx11-dev libxcursor-dev libxrandr-dev \
			libxinerama-dev libxi-dev libxxf86vm-dev libgl1-mesa-dev; \
	elif command -v pacman >/dev/null 2>&1; then \
		sudo pacman -S --needed libx11 libxcursor libxrandr libxinerama \
			libxi libxxf86vm mesa; \
	else \
		printf "$(RED)Unknown package manager$(RESET)\n"; \
		exit 1; \
	fi
	@printf "$(GREEN)Dependencies installed!$(RESET)\n"

test: ## Run unit tests (no external deps)
	@printf "$(BLUE)Running unit tests...$(RESET)\n"
	@go test -v -count=1 -race ./...
	@printf "$(GREEN)Done!$(RESET)\n"

test-x11: ## Run X11 integration tests (needs DISPLAY + PulseAudio)
	@printf "$(BLUE)Running X11 integration tests...$(RESET)\n"
	@go test -v -count=1 -tags x11test ./internal/typing/... ./internal/audio/...
	@printf "$(GREEN)Done!$(RESET)\n"

test-integration: ## Run distro container tests (requires Docker)
	@printf "$(BLUE)Running distro container tests...$(RESET)\n"
	@go test -v -count=1 -tags integration -timeout 45m ./tests/integration/...
	@printf "$(GREEN)Done!$(RESET)\n"

test-all: test test-integration ## Run unit + distro container tests

tidy: ## Tidy and verify Go modules
	@printf "$(BLUE)Tidying modules...$(RESET)\n"
	@go mod tidy
	@go mod verify
	@printf "$(GREEN)Done!$(RESET)\n"

fmt: ## Format all Go source files
	@printf "$(BLUE)Formatting...$(RESET)\n"
	@go fmt ./...
	@printf "$(GREEN)Done!$(RESET)\n"

vet: ## Run go vet
	@printf "$(BLUE)Vetting...$(RESET)\n"
	@go vet ./...
	@printf "$(GREEN)Done!$(RESET)\n"

lint: vet ## Run golangci-lint (install from https://golangci-lint.run if missing)
	@command -v golangci-lint >/dev/null 2>&1 || \
		{ printf "$(RED)golangci-lint not found — install from https://golangci-lint.run$(RESET)\n"; exit 1; }
	@printf "$(BLUE)Linting...$(RESET)\n"
	@golangci-lint run ./...
	@printf "$(GREEN)Done!$(RESET)\n"

# --- Help ---------------------------------------------------

help: ## Show this help message
	@printf "$(CYAN)GoTalk Dictation$(RESET)\n\n"
	@printf "Usage:\n"
	@printf "  $(YELLOW)make$(RESET) $(GREEN)<target>$(RESET)\n\n"
	@printf "Targets:\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(YELLOW)%-12s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
