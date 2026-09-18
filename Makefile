GO ?= go
BINARY ?= stickit
BIN_DIR := bin

.PHONY: all build test vet fmt check install clean

all: build

build:
	$(GO) build -o $(BIN_DIR)/$(BINARY) .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

check: vet test
	./scripts/run_checks.sh

install:
	$(GO) install .

clean:
	rm -rf $(BIN_DIR)
