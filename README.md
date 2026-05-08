# VSIX Downloader

A small Go based CLI tool for downloading VS Code extensions as `.vsix` files from either
the **VS Code Marketplace** or **Open VSX**.

## Build

```bash
# Build for current platform.
make build
./bin/vsixdl --help

# Or install into $GOPATH/bin.
make install

# Or build cross-platform release binaries (linux/darwin/windows, amd64/arm64).
make release
ls dist/
```

Single static binary by default (`CGO_ENABLED=0`).

## Usage

```bash
# Latest stable version from the Microsoft VS Code Marketplace (default source).
vsixdl get ms-python.python

# Download a specific version.
vsixdl get --version 2024.22.0 ms-python.python

# Download extension from Open VSX instead.
vsixdl --source openvsx download golang.go

# Download several extensions at once into a bundle directory.
vsixdl get -o ./bundle \
    ms-azuretools.vscode-docker \
    redhat.ansible \
    rust-lang.rust-analyzer

# From a list file (one per line, '#' comments, optional '@version' suffix).
vsixdl get --from-file extensions.txt --output ./bundle

# Platform-specific build (rust-analyzer, native dependencies, etc).
vsixdl get --target-platform linux-x64 rust-lang.rust-analyzer

# Show metadata.
vsixdl info ms-python.python

# List the most recent 20 versions.
vsixdl list ms-python.python
vsixdl list -n 0 ms-python.python  # all of them
```

### Example list file

```text
# Editor essentials
editorconfig.editorconfig
eamodio.gitlens

# Pinned versions
ms-python.python@2024.22.0

# Languages
golang.go
rust-lang.rust-analyzer
```

### Common target platforms

`linux-x64`, `linux-arm64`, `darwin-x64`, `darwin-arm64`,
`win32-x64`, `win32-arm64`, `alpine-x64`, `alpine-arm64`.

Leave empty for the universal (platform-neutral) build, which is what most
extensions ship.

## Flags

```text
Global:
  -s, --source       ms-marketplace | openvsx        (default: ms-marketplace)
  -q, --quiet        suppress progress output
  -V, --version      print version

download [extensions...]:
  -f, --from-file    read identifiers from a file
  -v, --version      version to fetch              (default: latest)
  -o, --output       output directory              (default: .)
      --target-platform  e.g. linux-x64
      --pre-release  allow pre-release versions
      --force        overwrite existing files

info <extension>:
  (no extra flags)

versions <extension>:
  -n, --limit        max entries to show           (default: 20, 0 = all)
```

## Layout

```text
vsixdl/
├── main.go        # entrypoint
├── client.go      # registry interface + MsMarketplace and OpenVSX clients
├── download.go    # download command, batch logic, progress meter
├── info.go        # info + versions subcommands
├── go.mod
├── Makefile
└── README.md
```

## A licensing note on the Microsoft Marketplace

Microsoft's Marketplace terms restrict the use of Microsoft-published extensions
to genuine Microsoft builds of VS Code. That is precisely why VSCodium and
similar forks default to Open VSX. Use the right source for your situation:

- Genuine VS Code, offline install: Microsoft's marketplace is fine.
- VSCodium / Cursor / Kiro IDE: prefer Open VSX. Microsoft's closed-source and 
  proprietary extensions (Pylance, the official C/C++ pack, Remote-SSH, etc.) 
  are not mirrored there by design.
