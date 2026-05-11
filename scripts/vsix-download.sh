#!/bin/bash
set -euo pipefail

# vsix-download.sh

EXTENSIONS=(
  "ms-python.python"
  "ms-azuretools.vscode-docker"
  "redhat.ansible"
  "golang.go"
  "rust-lang.rust-analyzer"
)

OUT_DIR="${HOME}/vscode-extensions"
mkdir -p "${OUT_DIR}"

function resolve_extension_version() {
  local item="$1"
  curl -s -X POST \
    -H "Accept: application/json;api-version=3.0-preview.1" \
    -H "Content-Type: application/json" \
    -d "{\"filters\":[{\"criteria\":[{\"filterType\":7,\"value\":\"${item}\"}]}],\"flags\":914}" \
    https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery |
    python3 -c 'import sys, json; print(json.load(sys.stdin)["results"][0]["extensions"][0]["versions"][0]["version"])'
}

for item in "${EXTENSIONS[@]}"; do
  publisher="${item%%.*}"
  extension="${item#*.}"
  version="$(resolve_extension_version "${item}")"
  out_file="${OUT_DIR}/${publisher}.${extension}-${version}.vsix"

  if [[ -f "${out_file}" ]]; then
    echo "skip: ${item} ${version}"
    continue
  fi

  echo "Download: ${item} ${version}"
  curl -sSfL -o "${out_file}" \
    "https://${publisher}.gallery.vsassets.io/_apis/public/gallery/publisher/${publisher}/extension/${extension}/${version}/assetbyname/Microsoft.VisualStudio.Services.VSIXPackage"
done

echo "done: ${OUT_DIR}"
