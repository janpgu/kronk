BINARY  = kronk
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: build build-all clean run

## build: compile for the current OS/arch
build:
	go build $(LDFLAGS) -o $(BINARY) .

## build-all: cross-compile for Windows, Linux, and macOS
build-all:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/kronk-windows-amd64.exe .
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/kronk-linux-amd64 .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/kronk-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/kronk-darwin-amd64 .

## clean: remove compiled binaries
clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/

## run: build and run with verbose tick
run: build
	./$(BINARY) run
