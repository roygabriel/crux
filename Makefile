BINARY := crux
BUILD_DIR := bin
MODULE := github.com/roygabriel/crux

LDFLAGS := -s -w
BUILDFLAGS := -trimpath -ldflags="$(LDFLAGS)"

MIN_COVERAGE := 70

.PHONY: build test lint vet coverage coverage-check integration clean

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

clean:
	rm -rf $(BUILD_DIR) coverage.out
