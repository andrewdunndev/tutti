SHELL := /bin/bash

# Single entry point for every aspect of the project. Five concerns,
# grouped below: binary, data, capture, brand, site. Plus one
# top-level pre-tag verifier (release-check) and a few utilities.

.PHONY: help \
        build test vet tidy lint \
        sync-schema gen-tones \
        capture validate \
        brand \
        site site-dev site-preview site-install site-clean \
        release-check clean

GO  ?= go
BIN ?= tutti

# --- help -----------------------------------------------------------

## help: list every target, grouped by concern
help:
	@echo ""
	@echo "tutti make targets"
	@echo ""
	@echo "binary:"
	@echo "  build           compile ./$(BIN) from cmd/tutti (runs sync-schema first)"
	@echo "  test            go test -race ./..."
	@echo "  vet             go vet ./..."
	@echo "  tidy            go mod tidy"
	@echo "  lint            golangci-lint run ./... (if installed)"
	@echo ""
	@echo "data:"
	@echo "  sync-schema     copy schema/manifest.v1.json into internal/schema/ for go:embed"
	@echo "  gen-tones       regenerate the embedded test-tone matrix (requires ffmpeg)"
	@echo ""
	@echo "capture:"
	@echo "  capture         ./$(BIN) capture (builds first if needed)"
	@echo "  validate DIR=X  ./$(BIN) validate <DIR>"
	@echo ""
	@echo "brand:"
	@echo "  brand           regenerate avatar.png + brand.png (requires rsvg-convert + magick)"
	@echo ""
	@echo "site (web/):"
	@echo "  site-install    npm ci in web/ (reproducible)"
	@echo "  site            astro build → web/dist/"
	@echo "  site-dev        astro dev (live reload)"
	@echo "  site-preview    site + astro preview"
	@echo "  site-clean      remove web/dist/ and web/.astro/"
	@echo ""
	@echo "top-level:"
	@echo "  release-check   pre-tag verification (cross-build all targets + test + vet)"
	@echo "  clean           remove every build output"
	@echo ""

# --- binary ---------------------------------------------------------

build: sync-schema
	$(GO) build -o $(BIN) ./cmd/tutti

test: sync-schema
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

lint:
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not installed; skipping"; exit 0; }
	golangci-lint run ./...

# --- data -----------------------------------------------------------

sync-schema:
	@cp schema/manifest.v1.json internal/schema/manifest.v1.json

gen-tones:
	@bash scripts/gen-tones.sh internal/audio/tones

# --- capture --------------------------------------------------------

capture: build
	./$(BIN) capture

validate: build
	./$(BIN) validate $(DIR)

# --- brand ----------------------------------------------------------

# brand: regenerate avatar.png and brand.png from avatar.svg + hero.svg.
# Convention documented at design.dunn.dev/repo-brand.
brand:
	@command -v rsvg-convert >/dev/null || { echo "rsvg-convert (librsvg) is required"; exit 1; }
	@command -v magick >/dev/null || { echo "magick (ImageMagick) is required"; exit 1; }
	rsvg-convert -w 256 -h 256 -o avatar.png avatar.svg
	rsvg-convert -w 1600 -h 573 -o /tmp/tutti-brand-hero.png hero.svg
	rsvg-convert -w 800 -h 800 -o /tmp/tutti-brand-avatar.png avatar.svg
	magick /tmp/tutti-brand-avatar.png -background '#f4f4f4' -gravity center -extent 1600x800 /tmp/tutti-brand-avatar-pad.png
	magick /tmp/tutti-brand-avatar-pad.png /tmp/tutti-brand-hero.png -append -bordercolor '#f4f4f4' -border 24 brand.png
	@rm -f /tmp/tutti-brand-*.png
	@echo "brand: avatar.png ($$(wc -c < avatar.png) bytes), brand.png ($$(wc -c < brand.png) bytes)"

# --- site -----------------------------------------------------------

# The static corpus site lives at web/. It's an Astro project with a
# content collection backed by ../evidence/*/captures/*/manifest.json,
# rendering an index of devices and per-device capture pages. Site
# deploys to tutti.dunn.dev via Cloudflare Pages.
#
# All site-* targets cd into web/. The repo's Makefile is still the
# single entry point.

site-install:
	cd web && npm ci

site: site-install
	cd web && npm run build

site-dev:
	@[ -d web/node_modules ] || (cd web && npm install)
	cd web && npm run dev

site-preview: site
	cd web && npm run preview

site-clean:
	rm -rf web/dist web/.astro

# --- top-level ------------------------------------------------------

# release-check: the pre-tag battery. Cross-build the four release
# targets (linux/amd64, linux/arm64, darwin/arm64, windows/amd64),
# run tests, and vet. Mirrors what CI does, but locally and fast.
release-check: sync-schema test vet
	@echo "==> cross-build matrix"
	@mkdir -p /tmp/tutti-release-check
	GOOS=linux   GOARCH=amd64 $(GO) build -trimpath -o /tmp/tutti-release-check/tutti-linux-amd64       ./cmd/tutti
	GOOS=linux   GOARCH=arm64 $(GO) build -trimpath -o /tmp/tutti-release-check/tutti-linux-arm64       ./cmd/tutti
	GOOS=darwin  GOARCH=arm64 $(GO) build -trimpath -o /tmp/tutti-release-check/tutti-darwin-arm64      ./cmd/tutti
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -o /tmp/tutti-release-check/tutti-windows-amd64.exe ./cmd/tutti
	@ls -la /tmp/tutti-release-check/
	@rm -rf /tmp/tutti-release-check
	@echo "==> all four targets cross-built; release-check OK"

clean: site-clean
	rm -f $(BIN) avatar.png brand.png
	rm -rf bin/ dist/

