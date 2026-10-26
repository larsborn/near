# NEAR — FAR Manager-style dual pane file manager.
#
# Targets mirror the commands in CLAUDE.md, README.md and the GitHub workflows:
#
#   make            build the native binary (near, or near.exe on Windows)
#   make check      gofmt -l, go vet and the test suite — run before every commit
#   make cross      compile every release target to /dev/null (what CI does)
#   make dist       build the release archives into dist/ (what release.yml does)
#   make clean      remove binaries, build/ and dist/

GO      ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X main.version=$(VERSION)

# Native binary name: near.exe on Windows, near elsewhere.
ifeq ($(OS),Windows_NT)
BIN := near.exe
else
BIN := near
endif

# Every target release.yml publishes and ci.yml cross-compiles.
TARGETS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64

.PHONY: all build install run defaults fmt fmt-check vet test check cross dist clean

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	$(GO) install -ldflags "$(LDFLAGS)" .

run: build
	./$(BIN)

# Print the current default config (the embedded TOML text in config.go).
defaults: build
	./$(BIN) -defaults

fmt:
	gofmt -w .

# Fails if any file is not gofmt-clean, listing the offenders.
fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

check: fmt-check vet test

# Cross-compile convenience builds for the two platforms named in CLAUDE.md.
near-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ .

near-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ .

# Compile every release target without keeping the output, exactly like CI.
cross:
	@for target in $(TARGETS); do \
		GOOS=$${target%/*} GOARCH=$${target#*/} CGO_ENABLED=0 $(GO) build -o /dev/null . || exit 1; \
		echo "ok $$target"; \
	done

# Build the release archives the way release.yml does: one directory per
# target under build/, zipped (Windows) or tarred (everything else) into dist/.
dist:
	@set -e; mkdir -p dist; \
	for target in $(TARGETS); do \
		os=$${target%/*}; arch=$${target#*/}; \
		bin=near; [ "$$os" = windows ] && bin=near.exe; \
		dir="near-$(VERSION)-$$os-$$arch"; \
		mkdir -p "build/$$dir"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			$(GO) build -trimpath -ldflags "-s -w $(LDFLAGS)" -o "build/$$dir/$$bin" .; \
		cp README.md "build/$$dir/"; [ -f LICENSE ] && cp LICENSE "build/$$dir/" || true; \
		if [ "$$os" = windows ]; then \
			(cd build && zip -qr "../dist/$$dir.zip" "$$dir"); \
		else \
			tar -C build -czf "dist/$$dir.tar.gz" "$$dir"; \
		fi; \
		echo "built $$dir"; \
	done; \
	(cd dist && sha256sum * > SHA256SUMS.txt); \
	ls -l dist

clean:
	rm -rf near near.exe near-linux-* near-darwin-* build dist
