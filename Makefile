BINARY := clalt
INSTALL_DIR ?= $(HOME)/bin

# build metadata injected via -ldflags -X into internal/version; VERSION falls
# back to "dev" (the package default) when git isn't available or has no tags.
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null)
COMMIT  := $(shell git rev-parse HEAD 2>/dev/null)
DATE    := $(shell date -u +%Y-%m-%d)
VERSION_PKG := github.com/dhitalkamal/clalt/internal/version
LDFLAGS := -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).Date=$(DATE)
ifneq ($(strip $(VERSION)),)
LDFLAGS += -X $(VERSION_PKG).Version=$(VERSION)
endif

.PHONY: build test vet fmt fmtcheck check install cross clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmtcheck:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "unformatted:"; echo "$$out"; exit 1; fi

# run everything CI would
check: fmtcheck vet test build

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed to $(INSTALL_DIR)/$(BINARY) (ensure it is on your PATH)"

# cross builds ready-to-copy binaries for other devices into dist/. Every
# target is pure Go (CGO_ENABLED=0, no keychain/cgo dependency anywhere), so
# any of these can be built from any host, including macOS from Linux CI.
# Copy the matching dist/ file to the target's PATH; no Go needed there.
cross:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/clalt-darwin-amd64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/clalt-darwin-arm64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/clalt-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/clalt-linux-arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/clalt-windows-amd64.exe .
	@echo "built dist/ binaries:"; ls -1 dist

clean:
	rm -f $(BINARY)
	rm -rf dist
