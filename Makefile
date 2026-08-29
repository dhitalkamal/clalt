BINARY := clalt
INSTALL_DIR ?= $(HOME)/bin

# CODESIGN_IDENTITY is the name of a stable code-signing identity in your login
# keychain (a self-signed code-signing cert is fine; see the README). When set,
# build/install sign the binary with it so its code identity is stable across
# rebuilds and a macOS Keychain "always allow" grant persists instead of the
# login-password prompt recurring every run. When empty, `go build` leaves the
# default ad-hoc signature and clalt warns that prompts will recur.
CODESIGN_IDENTITY ?=
CODESIGN_ID := com.clalt.cli

.PHONY: build sign test vet fmt fmtcheck check install cross clean

build:
	go build -o $(BINARY) .
	@$(MAKE) --no-print-directory sign

# sign applies the stable identity when one is configured; otherwise it warns
# that the binary keeps its ad-hoc signature.
sign:
ifeq ($(strip $(CODESIGN_IDENTITY)),)
	@echo "warning: no CODESIGN_IDENTITY set; binary is ad-hoc signed and macOS will re-prompt for your login password every run."
	@echo "         build with 'make install CODESIGN_IDENTITY=<name>' once you have created the identity (see README)."
else
	codesign --force --identifier $(CODESIGN_ID) --sign "$(CODESIGN_IDENTITY)" $(BINARY)
	@echo "signed $(BINARY) with identity '$(CODESIGN_IDENTITY)'"
endif

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

# cross builds ready-to-copy binaries for other devices into dist/. Linux and
# Windows are pure Go (no cgo); the macOS binary is built for this Mac's arch
# (other Macs should build from source so signing is local). Copy the matching
# dist/ file to the target's PATH; no Go needed on the target.
cross:
	@mkdir -p dist
	go build -o dist/clalt-darwin-$$(go env GOARCH) .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/clalt-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/clalt-linux-arm64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/clalt-windows-amd64.exe .
	@echo "built dist/ binaries:"; ls -1 dist

clean:
	rm -f $(BINARY)
	rm -rf dist
