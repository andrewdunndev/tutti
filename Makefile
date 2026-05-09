.PHONY: build test tidy sync-schema gen-tones capture validate brand help

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
	@echo "  brand        - regenerate avatar.png + brand.png from avatar.svg + hero.svg"

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

# brand: regenerate avatar.png and brand.png from avatar.svg + hero.svg.
# Requires rsvg-convert (librsvg) and ImageMagick (magick). Convention
# is documented at design.dunn.dev/repo-brand.
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
