BINARY_NAME=scheduler
BUILD_DIR=./build
CMD_DIR=./cmd/scheduler
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build clean test install install-skill install-agents

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

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
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

install-skill:
	@mkdir -p $(HOME)/.claude/skills/schedule
	cp .workspace/.agents/skills/schedule/SKILL.md $(HOME)/.claude/skills/schedule/SKILL.md
	@echo "Skill installed to ~/.claude/skills/schedule/SKILL.md"
