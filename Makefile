VERSION := $(shell git describe --abbrev=0 --tags --always)
LDFLAGS := -X main.Version=$(VERSION)

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n "s/^##//p" ${MAKEFILE_LIST} | column -t -s ":" |  sed -e "s/^/ /"

.PHONY: confirm
confirm:
	@echo "Are you sure? (y/n) \c"
	@read answer; \
	if [ "$$answer" != "y" ]; then \
		echo "Aborting."; \
		exit 1; \
	fi

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## audit: run quality control checks
.PHONY: audit
audit:
	@echo "Checking module dependencies"
	go mod tidy -diff
	go mod verify
	@echo "Vetting code..."
	test -z "$(shell gofmt -l .)" 
	go vet ./...
	go tool staticcheck -checks=all,-ST1000,-U1000 ./...
	go tool govulncheck ./...
	@echo "Running tests..."
	go test -v -race -vet=off ./...

## test: run all tests
.PHONY: test
test:
	go test -v -race -buildvcs ./...

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## tidy: tidy and format all .go files
.PHONY: tidy
tidy:
	@echo "Tidying module dependencies..."
	go mod tidy
	@echo "Formatting .go files..."
	go fmt ./...

## build/keylightctl: build the cmd/keylightctl application
.PHONY: build/keylightctl
build/keylightctl:
	@go build -v -ldflags "$(LDFLAGS)" -o=./keylightctl .

## run/keylightctl: run the cmd/keylightctl application
.PHONY: run/keylightctl
run/keylightctl: build/keylightctl
	@./keylightctl

# vim: set tabstop=4 shiftwidth=4 noexpandtab
