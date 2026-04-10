APP_NAME := redis-stats
CMD_PATH := ./cmd/redis-stats
DIST_DIR := dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
RELEASE_NOTES_FILE ?=
RELEASE_TEMPLATE := .github/RELEASE_TEMPLATE.md
GENERATED_RELEASE_NOTES := $(DIST_DIR)/RELEASE_NOTES_$(VERSION).md

.PHONY: help run-cli run-watch run-server run-audit test build fmt clean check build-release release-notes publish-release

help:
	@echo "Targets:"
	@echo "  make run-cli        Run snapshot mode against REDIS_URL"
	@echo "  make run-watch      Run watch mode against REDIS_URL"
	@echo "  make run-server     Run dashboard server against REDIS_URL"
	@echo "  make run-audit      Run exhaustive TTL audit against REDIS_URL"
	@echo "  make fmt            Format Go code"
	@echo "  make test           Run go test ./..."
	@echo "  make check          Run fmt, test, and build"
	@echo "  make build          Build the local binary"
	@echo "  make clean          Remove local build artifacts"
	@echo "  make build-release  Build release archives into $(DIST_DIR)/"
	@echo "  make release-notes  Render release notes from $(RELEASE_TEMPLATE)"
	@echo "  make publish-release Publish release archives with gh for VERSION=$(VERSION)"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  RELEASE_NOTES_FILE=/path/to/release-notes.md"

run-cli:
	go run $(CMD_PATH) snapshot

run-watch:
	go run $(CMD_PATH) watch

run-server:
	go run $(CMD_PATH) serve

run-audit:
	go run $(CMD_PATH) ttl-audit

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

check: fmt test build

build:
	go build -o $(APP_NAME) ./cmd/redis-stats

clean:
	rm -rf $(DIST_DIR)
	rm -f $(APP_NAME)
	rm -f $(APP_NAME).exe

build-release: clean
	mkdir -p $(DIST_DIR)
	@set -eu; \
	for target in "darwin amd64" "darwin arm64" "linux amd64" "linux arm64"; do \
		set -- $$target; \
		os=$$1; arch=$$2; \
		name="$(APP_NAME)_$(VERSION)_$${os}_$${arch}"; \
		outdir="$(DIST_DIR)/$$name"; \
		mkdir -p "$$outdir"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -o "$$outdir/$(APP_NAME)" ./cmd/redis-stats; \
		cp README.md LICENSE "$$outdir/"; \
		tar -C "$(DIST_DIR)" -czf "$(DIST_DIR)/$$name.tar.gz" "$$name"; \
		rm -rf "$$outdir"; \
	done
	@set -eu; \
	for target in "windows amd64"; do \
		set -- $$target; \
		os=$$1; arch=$$2; \
		name="$(APP_NAME)_$(VERSION)_$${os}_$${arch}"; \
		outdir="$(DIST_DIR)/$$name"; \
		mkdir -p "$$outdir"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -o "$$outdir/$(APP_NAME).exe" ./cmd/redis-stats; \
		cp README.md LICENSE "$$outdir/"; \
		(cd "$(DIST_DIR)" && zip -qr "$$name.zip" "$$name"); \
		rm -rf "$$outdir"; \
	done
	@echo "Release artifacts created in $(DIST_DIR)/"

release-notes:
	mkdir -p $(DIST_DIR)
	sed 's/{{VERSION}}/$(VERSION)/g' "$(RELEASE_TEMPLATE)" > "$(GENERATED_RELEASE_NOTES)"
	@echo "Rendered release notes: $(GENERATED_RELEASE_NOTES)"

publish-release: build-release release-notes
	@if ! command -v gh >/dev/null 2>&1; then \
		echo "gh CLI is required for publish-release"; \
		exit 1; \
	fi
	@if [ -n "$(RELEASE_NOTES_FILE)" ]; then \
		gh release create "$(VERSION)" $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip --notes-file "$(RELEASE_NOTES_FILE)"; \
	else \
		gh release create "$(VERSION)" $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip --notes-file "$(GENERATED_RELEASE_NOTES)"; \
	fi
