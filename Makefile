BINARY  := fitmerge
PKG     := ./cmd/fitmerge
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet fmt install clean dist

all: vet test build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

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
