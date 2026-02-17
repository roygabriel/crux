BINARY := crux
BUILD_DIR := bin
MODULE := github.com/roygabriel/crux

LDFLAGS := -s -w
BUILDFLAGS := -trimpath -ldflags="$(LDFLAGS)"

.PHONY: build test lint vet coverage clean

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

integration:
	go test -race -tags=integration ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out
