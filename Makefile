BINARY  := fitmerge
PKG     := ./cmd/fitmerge
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PORT    ?= 8080

.PHONY: all build test vet fmt install clean dist web web-docker

all: vet test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

# Build the browser (WebAssembly) version into web/.
web:
	GOOS=js GOARCH=wasm go build -ldflags "$(LDFLAGS)" -o web/fitmerge.wasm ./cmd/fitmerge-wasm
	cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" web/
	@echo "Built web/. Serve it over HTTP, e.g.:  cd web && python -m http.server 8080"

# Build the browser (WebAssembly) UI into a container image and serve it, so
# the app can be hosted with just Docker (no Go toolchain or web server needed).
# Override the published port with PORT=... (default 8080).
web-docker:
	docker build -f Dockerfile.web --build-arg VERSION="$(VERSION)" -t fitmerge-web .
	@echo "Serving fitmerge web UI on http://localhost:$(PORT)/  (Ctrl-C to stop)"
	docker run --rm -p $(PORT):80 fitmerge-web

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist

# Cross-compile release binaries into dist/.
dist: clean
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   $(PKG)
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  $(PKG)
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  $(PKG)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(PKG)
