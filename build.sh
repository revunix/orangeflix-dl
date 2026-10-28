#!/usr/bin/env bash
#
# Baut die Binaries für alle Plattformen des Releases, packt sie und schreibt
# die Prüfsummen, wie es der Release-Workflow tut.
#
# Aufruf: ./build.sh [version]
#
# Ohne Argument gilt die Version aus "git describe", sonst "dev".
set -euo pipefail

cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
  echo "Go fehlt im PATH." >&2
  exit 1
fi
if ! command -v zip >/dev/null 2>&1; then
  echo "zip fehlt (wird für die Windows-Archive gebraucht)." >&2
  exit 1
fi

version="${1:-${VERSION:-}}"
if [[ -z "$version" ]] && git rev-parse --git-dir >/dev/null 2>&1; then
  version="$(git describe --tags --always --dirty 2>/dev/null || true)"
fi
if [[ -z "$version" ]]; then
  version="dev"
fi
# Der Tag steht im Dateinamen, Schrägstriche würden Pfade erzeugen.
archive_version="${version//\//-}"

dist="dist"
rm -rf "$dist"
mkdir -p "$dist"

# Die Plattformen des Release-Workflows.
targets=(
  linux/amd64
  linux/arm64
  darwin/amd64
  darwin/arm64
  windows/amd64
  windows/arm64
)

for target in "${targets[@]}"; do
  goos="${target%%/*}"
  goarch="${target##*/}"

  binary="orangeflix-dl"
  if [[ "$goos" == "windows" ]]; then
    binary="orangeflix-dl.exe"
  fi

  printf 'baue  %-16s ' "${goos}/${goarch}"
  # CGO bleibt aus: SQLite kommt aus modernc.org/sqlite, es braucht keine
  # C-Bibliothek und damit keine fremde Werkzeugkette.
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "-s -w -X main.version=${version}" \
    -o "$dist/$binary" ./cmd/orangeflix-dl

  base="orangeflix-dl_${archive_version}_${goos}_${goarch}"
  if [[ "$goos" == "windows" ]]; then
    (cd "$dist" && zip -q "${base}.zip" "$binary")
    size="$(wc -c < "$dist/${base}.zip" | tr -d ' ')"
  else
    tar -czf "$dist/${base}.tar.gz" -C "$dist" "$binary"
    size="$(wc -c < "$dist/${base}.tar.gz" | tr -d ' ')"
  fi
  rm -f "$dist/$binary"
  printf '%s Bytes\n' "$size"
done

# Prüfsummen im selben Format wie im Release.
sum_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}
(
  cd "$dist"
  : > checksums.txt
  for file in *; do
    if [[ "$file" == "checksums.txt" ]]; then
      continue
    fi
    printf '%s  %s\n' "$(sum_file "$file")" "$file" >> checksums.txt
  done
)

printf '\nfertig: %s\n' "$dist"
ls -lh "$dist"
