MODULE := github.com/chrisophus/clipd
CMDS   := clip clipd
BIN    := bin

# Version: use git tag if available, otherwise dev.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: all build clean test install release snapshot

all: build

## build: compile clip and clipd into bin/
build:
	@mkdir -p $(BIN)
	$(foreach cmd,$(CMDS),go build -ldflags "$(LDFLAGS)" -o $(BIN)/$(cmd) ./cmd/$(cmd) &&) true

## test: run all tests
test:
	go test ./...

## install: install clip and clipd into GOPATH/bin (or ~/go/bin)
install:
	$(foreach cmd,$(CMDS),go install -ldflags "$(LDFLAGS)" ./cmd/$(cmd) &&) true

## clean: remove compiled binaries
clean:
	rm -rf $(BIN)

## release: tag HEAD as vX.Y.Z and push (usage: make release VERSION=1.2.3)
release:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "usage: make release VERSION=1.2.3"; exit 1; fi
	@echo "Tagging v$(VERSION) and pushing..."
	git tag -a "v$(VERSION)" -m "Release v$(VERSION)"
	git push origin "v$(VERSION)"

## snapshot: build a local GoReleaser snapshot (no publish)
snapshot:
	goreleaser release --snapshot --clean
