VERSION := $(shell git describe --abbrev=0 --tags --always)
LDFLAGS := -X main.Version=$(VERSION)
BIN := ./bin/keylightctl
BINDIR ?= $(HOME)/.local/bin

## help: print this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n "s/^##//p" ${MAKEFILE_LIST} | column -t -s ":" | sed -e "s/^/ /"

## audit: run quality control checks
.PHONY: audit
audit:
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)"
	go vet ./...

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

## tidy: tidy and format all .go files
.PHONY: tidy
tidy:
	go mod tidy
	go fmt ./...

## build: build the application
.PHONY: build
build:
	@go build -v -ldflags "$(LDFLAGS)" -o=$(BIN) . 2>/dev/null

## install: build and install to BINDIR (default ~/.local/bin)
.PHONY: install
install: build
	@mkdir -p $(BINDIR)
	@cp $(BIN) $(BINDIR)/keylightctl
	@echo "Installed keylightctl to $(BINDIR)/keylightctl"

## uninstall: remove the installed binary from BINDIR
.PHONY: uninstall
uninstall:
	@rm -f $(BINDIR)/keylightctl
	@echo "Removed $(BINDIR)/keylightctl"

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -rf bin


# vim: set tabstop=4 shiftwidth=4 noexpandtab
