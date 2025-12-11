OUT_DIR = bin
BIN = $(OUT_DIR)/local-registry
MAIN = main.go

# Version info (override with: make build VERSION=1.2.3)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")

LDFLAGS = -s -w \
	-X local-registry/cmd.Version=$(VERSION) \
	-X local-registry/cmd.Commit=$(COMMIT)

.PHONY: build clean install version

build:
	mkdir -p $(OUT_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(MAIN)

# Development build (no stripping, faster)
dev:
	mkdir -p $(OUT_DIR)
	go build -o $(BIN) $(MAIN)

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -rf $(OUT_DIR)

version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
