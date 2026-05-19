VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build install test lint clean tidy

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/otx ./cmd/otx

install:
	go install -ldflags "-s -w -X main.version=$(VERSION)" ./cmd/otx

test:
	go test -v ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/ dist/
