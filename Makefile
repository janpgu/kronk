VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

# On Windows use PowerShell so GOOS/GOARCH assignments work.
# On Unix the default sh is fine.
ifeq ($(OS),Windows_NT)
	BINARY = kronk.exe
	SHELL  = pwsh.exe
	.SHELLFLAGS = -NoProfile -NonInteractive -Command
else
	BINARY = kronk
endif

.PHONY: build build-all clean run

## build: compile for the current OS/arch
build:
	go build $(LDFLAGS) -o $(BINARY) .

## build-all: cross-compile for Windows, Linux, and macOS
build-all:
ifeq ($(OS),Windows_NT)
	New-Item -ItemType Directory -Force -Path dist | Out-Null
	$$env:GOOS='windows'; $$env:GOARCH='amd64'; go build $(LDFLAGS) -o dist/kronk-windows-amd64.exe . ; Write-Host "Built kronk-windows-amd64.exe"
	$$env:GOOS='linux';   $$env:GOARCH='amd64'; go build $(LDFLAGS) -o dist/kronk-linux-amd64        . ; Write-Host "Built kronk-linux-amd64"
	$$env:GOOS='darwin';  $$env:GOARCH='arm64'; go build $(LDFLAGS) -o dist/kronk-darwin-arm64       . ; Write-Host "Built kronk-darwin-arm64"
	$$env:GOOS='darwin';  $$env:GOARCH='amd64'; go build $(LDFLAGS) -o dist/kronk-darwin-amd64       . ; Write-Host "Built kronk-darwin-amd64"
else
	mkdir -p dist
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/kronk-windows-amd64.exe .
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/kronk-linux-amd64        .
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/kronk-darwin-arm64        .
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/kronk-darwin-amd64        .
endif

## clean: remove compiled binaries
clean:
ifeq ($(OS),Windows_NT)
	Remove-Item -ErrorAction SilentlyContinue $(BINARY)
	Remove-Item -Recurse -ErrorAction SilentlyContinue dist
else
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/
endif

## run: build and run
run: build
ifeq ($(OS),Windows_NT)
	./$(BINARY) run
else
	./$(BINARY) run
endif
