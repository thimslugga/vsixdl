#!/usr/bin/env just --justfile
# vim:set ft=just ts=2 sts=4 sw=2 et:

set positional-arguments
set allow-duplicate-recipes := false
set shell := ["bash", "-euo", "pipefail", "-c"]
set windows-shell := ["powershell.exe", "-NoLogo", "-Command"]

binary := "vsixdl"
pkg := "."
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
ldflags := "-s -w -X main.version=" + version
goflags := "-trimpath"

[doc('Lists the tasks and variables in the justfile')]
@_list:
    just --justfile {{ justfile() }} --list --unsorted
    echo ""
    echo "Available variables:"
    just --evaluate | sed 's/^/    /'
    echo ""
    echo "Override variables using 'just key=value ...' (also ALL_UPPERCASE ones)"

[doc('Evaluate and return all just variables')]
evaluate:
    @just --evaluate

[doc('List available recipes')]
help:
    @just --justfile {{ justfile() }} --list

[doc('Just format')]
format:
    just --justfile {{ justfile() }} --fmt

[doc('Return system information')]
system-info:
    @echo "os: {{ os() }}"
    @echo "family: {{ os_family() }}"
    @echo "architecture: {{ arch() }}"
    @echo "home directory: {{ home_directory() }}"
    @echo "project directory: {{ justfile_directory() }}"

[doc('Format go code')]
fmt:
    gofmt -w .

[doc('Run go vet')]
vet:
    go vet ./...

[doc('Tidy go modules')]
tidy:
    go mod tidy

[doc('Run tests')]
test:
    go test ./...

[doc('Run vsixdl with arguments')]
run *args:
    go run {{ goflags }} -ldflags "{{ ldflags }}" {{ pkg }} {{ args }}

[doc('Build the binary')]
build:
    CGO_ENABLED=0 go build {{ goflags }} -ldflags "{{ ldflags }}" -o bin/{{ binary }} {{ pkg }}

[doc('Remove build artifacts')]
clean:
    rm -rf bin/ dist/

[doc('Install into $GOPATH/bin')]
install:
    CGO_ENABLED=0 go install {{ goflags }} -ldflags "{{ ldflags }}" {{ pkg }}

[doc('Create release binaries for all supported platforms')]
release: clean
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p dist
    for target in linux/amd64 linux/arm64 darwin/arm64 windows/amd64; do
        os="${target%/*}"; arch="${target#*/}"
        ext=""; [[ "$os" == "windows" ]] && ext=".exe"
        out="dist/{{ binary }}-{{ version }}-${os}-${arch}${ext}"
        echo "build $out"
        CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
            go build {{ goflags }} -ldflags "{{ ldflags }}" -o "$out" {{ pkg }}
    done
    cd dist && shasum -a 256 {{ binary }}-* > SHA256SUMS

# Tag and push a release (e.g. just tag v1.0.0)
tag version:
    git tag {{ version }}
    git push origin {{ version }}
