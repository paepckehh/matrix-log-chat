PROJECT  := $(shell basename $(CURDIR))
MODULE   := paepcke.de/$(PROJECT)
VERSION  ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0-dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DIRTY    := $(shell git diff --quiet 2>/dev/null || echo -dirty)
BUILD    := $(COMMIT)$(DIRTY)
LDFLAGS  := -X $(MODULE)/version.Version=$(VERSION) -X $(MODULE)/version.Build=$(BUILD)

GOFLAGS  ?= -trimpath
CGO     ?= 0

all: build

info:
	@echo "$(PROJECT) $(VERSION) (build $(BUILD))"

version:
	@echo "$(VERSION)"

build:
	CGO_ENABLED=$(CGO) go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(PROJECT) .

run: build
	 MATRIX_HOMESERVER="https://matrix.debitor.de" \
	 MATRIX_USER="..." \
	 MATRIX_TOKEN="..." \
	 MATRIX_ROOM="!test" \
	 ./$(PROJECT)

test:
	go test ./...

check:
	go fmt -w -s .
	go vet ./...
	go fix ./...
	go test ./...

# Bump the patch segment of the latest git tag (v0.0.N -> v0.0.N+1) and
# commit+tag in one step. This is the project's only release path; see
# AGENTS.md "Tag" step. Run AFTER gofmt/build/test are already green.
patch:
	@NEXT=$$(git tag --list 'v*' --sort=-v:refname | head -1 | awk -F. '{printf "v0.0.%d", $$3+1}'); \
	echo "tagging $$NEXT"; \
	git tag $$NEXT

deps:
	touch go.mod go.sum
	rm go.mod go.sum
	go mod init $(MODULE)
	go mod tidy -v
	git config core.fileMode false

.PHONY: all info version build run test check patch deps
