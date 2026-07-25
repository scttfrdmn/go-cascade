BINARY := go-cascade
PKG    := ./...
GOBIN  := $(CURDIR)/bin

.PHONY: all build install test test-short check fmt vet lint demo calibrate-demo clean

all: check

build:
	@mkdir -p $(GOBIN)
	go build -o $(GOBIN)/$(BINARY) ./cmd/$(BINARY)

install:
	go install ./cmd/$(BINARY)

# The suite compiles and executes generated Go, so it needs a toolchain on PATH.
test:
	go test $(PKG) -timeout 30m

test-short:
	go test $(PKG) -short -timeout 5m

fmt:
	gofmt -w .

vet:
	go vet $(PKG)

lint:
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| echo "golangci-lint not installed; skipping (see .golangci.yml)"

check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	@$(MAKE) vet
	@$(MAKE) lint
	@$(MAKE) test

# End-to-end run with no AWS credentials required.
demo: build
	$(GOBIN)/$(BINARY) solve --provider=mock --mutants=8 \
		"Return the length of the longest strictly increasing contiguous run in a slice of integers."

# Profile every tier over the example problems, then emit a certificate.
calibrate-demo: build
	$(GOBIN)/$(BINARY) calibrate --provider=mock -bench examples/problems.jsonl \
		-alpha 0.15 -delta 0.10 -o thresholds.json -records records.json

clean:
	rm -rf $(GOBIN) thresholds.json records.json
