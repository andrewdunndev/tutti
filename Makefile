.PHONY: build test tidy sync-schema gen-tones capture validate help

GO ?= go
BIN ?= tutti

help:
	@echo "make targets:"
	@echo "  build        - compile the tutti binary into ./tutti"
	@echo "  test         - run the test suite"
	@echo "  tidy         - go mod tidy"
	@echo "  sync-schema  - copy schema/manifest.v1.json into internal/schema/ for go:embed"
	@echo "  gen-tones    - regenerate the embedded test-tone matrix (requires ffmpeg)"
	@echo "  capture      - run ./tutti capture (after build)"

build: sync-schema
	$(GO) build -o $(BIN) ./cmd/tutti

test: sync-schema
	$(GO) test ./...

tidy:
	$(GO) mod tidy

sync-schema:
	@cp schema/manifest.v1.json internal/schema/manifest.v1.json

gen-tones:
	@bash scripts/gen-tones.sh internal/audio/tones

capture: build
	./$(BIN) capture

validate: build
	./$(BIN) validate $(DIR)
