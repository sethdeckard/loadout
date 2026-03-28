VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/sethdeckard/loadout/cmd/loadout/cmd.version=$(VERSION) \
           -X github.com/sethdeckard/loadout/cmd/loadout/cmd.commit=$(COMMIT) \
           -X github.com/sethdeckard/loadout/cmd/loadout/cmd.date=$(DATE)

.PHONY: build test test-race cover cover-html lint vet clean

build:
	go build -ldflags "$(LDFLAGS)" -o loadout ./cmd/loadout

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

cover-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

vet:
	go vet ./...

lint:
	GOCACHE=$(CURDIR)/.cache/go-build GOLANGCI_LINT_CACHE=$(CURDIR)/.cache/golangci-lint golangci-lint run ./...

clean:
	rm -f loadout coverage.out
