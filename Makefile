BINARY_NAME=opencron
BUILD_DIR=./build
CMD_DIR=./cmd/opencron
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
PREFIX ?= /usr/local
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

ifeq ($(OS),Windows_NT)
	EXT=.exe
else
	EXT=
endif

.PHONY: build clean test install install-skill install-agents

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)$(EXT) $(CMD_DIR)

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

build-all: build-linux build-windows

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

install: build
ifeq ($(OS),Windows_NT)
	@echo "Windows detected — copying to $(GOPATH)\bin"
	@if not exist "$(GOPATH)\bin" mkdir "$(GOPATH)\bin"
	cp $(BUILD_DIR)/$(BINARY_NAME)$(EXT) $(GOPATH)/bin/$(BINARY_NAME)$(EXT)
else
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/$(BINARY_NAME)
endif

uninstall:
ifeq ($(OS),Windows_NT)
	rm -f $(GOPATH)/bin/$(BINARY_NAME)$(EXT)
else
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY_NAME)
endif

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

install-skill:
	@mkdir -p $(HOME)/.claude/skills/schedule
	cp .workspace/.agents/skills/schedule/SKILL.md $(HOME)/.claude/skills/schedule/SKILL.md
	@echo "Skill installed to ~/.claude/skills/schedule/SKILL.md"
