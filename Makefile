BINARY      := vsixdl
PKG         := .
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     := -trimpath
CGO_ENABLED ?= 0

.PHONY: build install test clean release fmt vet

build:
	CGO_ENABLED=$(CGO_ENABLED) go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf bin/ dist/

# Cross-compile static binaries for common platforms.
release: clean
	@mkdir -p dist
	@for target in \
	    linux/amd64 \
	    linux/arm64 \
	    darwin/amd64 \
	    darwin/arm64 \
	    windows/amd64 ; do \
	    os=$${target%/*} ; arch=$${target#*/} ; \
	    ext="" ; [ "$$os" = "windows" ] && ext=".exe" ; \
	    out="dist/$(BINARY)-$(VERSION)-$$os-$$arch$$ext" ; \
	    echo "build $$out" ; \
	    CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	      go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o "$$out" $(PKG) ; \
	done
	@cd dist && for f in $(BINARY)-*; do sha256sum "$$f" >> SHA256SUMS; done
	@ls -1 dist/
