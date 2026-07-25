BINARY := spin
PREFIX ?= /usr/local
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X github.com/cloudygreybeard/spin/cmd.Version=$(VERSION) \
	-X github.com/cloudygreybeard/spin/cmd.Commit=$(COMMIT) \
	-X github.com/cloudygreybeard/spin/cmd.Date=$(DATE)

.PHONY: all build test lint clean install snapshot deps help

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test -v -race ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY)
	rm -rf dist/

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

snapshot:
	goreleaser release --snapshot --clean

deps:
	go mod download
	go mod tidy

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build      Build the binary"
	@echo "  test       Run tests with race detector"
	@echo "  lint       Run golangci-lint"
	@echo "  clean      Remove build artifacts"
	@echo "  install    Install to PREFIX/bin (default: /usr/local/bin)"
	@echo "  snapshot   Build a snapshot release (no publish)"
	@echo "  deps       Download and tidy dependencies"
	@echo "  help       Show this help"
