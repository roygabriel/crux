BINARY := crux
BUILD_DIR := bin
MODULE := github.com/roygabriel/crux

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIRTY ?= $(shell test -n "$$(git status --porcelain 2>/dev/null)" && echo true || echo false)

LDFLAGS := -s -w \
	-X 'main.version=$(VERSION)' \
	-X 'main.commit=$(COMMIT)' \
	-X 'main.buildDate=$(BUILD_DATE)' \
	-X 'main.vcsDirty=$(DIRTY)'
BUILDFLAGS := -trimpath -ldflags="$(LDFLAGS)"

MIN_COVERAGE := 70

PREFIX ?= /usr/local

.PHONY: build test lint vet coverage coverage-check integration clean completions man docs-gen docs-serve docs-build install install-man

build:
	CGO_ENABLED=0 go build $(BUILDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/crux

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

coverage:
	go tool cover -func=coverage.out

coverage-check: coverage.out
	@echo "Checking coverage >= $(MIN_COVERAGE)%..."
	@total=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $$NF}' | tr -d '%'); \
	if [ "$$(echo "$$total < $(MIN_COVERAGE)" | bc -l)" = "1" ]; then \
		echo "FAIL: coverage $$total% is below minimum $(MIN_COVERAGE)%"; \
		exit 1; \
	else \
		echo "OK: coverage $$total% meets minimum $(MIN_COVERAGE)%"; \
	fi

coverage.out:
	$(MAKE) test

integration:
	go test -race -tags=integration ./...

completions: build
	@mkdir -p completions
	$(BUILD_DIR)/$(BINARY) completion bash > completions/crux.bash
	$(BUILD_DIR)/$(BINARY) completion zsh > completions/_crux
	$(BUILD_DIR)/$(BINARY) completion fish > completions/crux.fish

man: build
	$(BUILD_DIR)/$(BINARY) __gen-man --dir man/man1

docs-gen: build
	$(BUILD_DIR)/$(BINARY) __gen-docs --dir docs/site/content/reference/cli

docs-serve: docs-gen
	cd docs/site && hugo server -D

docs-build: docs-gen
	cd docs/site && hugo --minify

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY) $(DESTDIR)$(PREFIX)/bin/$(BINARY)

install-man: man
	install -d $(DESTDIR)$(PREFIX)/share/man/man1
	install -m 644 man/man1/*.1 $(DESTDIR)$(PREFIX)/share/man/man1/

clean:
	rm -rf $(BUILD_DIR) coverage.out
