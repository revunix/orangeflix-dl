#!/usr/bin/env bash
# Schreibt die Manifeste für Scoop und Nix für ein Release.
#
#   scripts/update-packages.sh v1.2.3 dist/checksums.txt
#
# Der Release-Workflow ruft das Skript nach dem Veröffentlichen auf.
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <tag> <checksums.txt>" >&2
  exit 2
fi

tag="$1"
checksums="$2"
version="${tag#v}"
repo="${GITHUB_REPOSITORY:-revunix/orangeflix-dl}"
root="$(cd "$(dirname "$0")/.." && pwd)"
desc="Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen"

sum_of() {
  local file="$1"
  local value
  value="$(awk -v f="$file" '$2 == f { print $1 }' "$checksums")"
  if [ -z "$value" ]; then
    echo "no checksum for $file in $checksums" >&2
    exit 1
  fi
  printf '%s' "$value"
}

asset() {
  local os="$1" arch="$2" ext="$3"
  printf 'orangeflix-dl_%s_%s_%s.%s' "$tag" "$os" "$arch" "$ext"
}

url_of() {
  printf 'https://github.com/%s/releases/download/%s/%s' "$repo" "$tag" "$1"
}

sri() {
  python3 -c 'import base64,sys; print("sha256-" + base64.b64encode(bytes.fromhex(sys.argv[1])).decode())' "$1"
}

win_amd64="$(sum_of "$(asset windows amd64 zip)")"
win_arm64="$(sum_of "$(asset windows arm64 zip)")"
linux_amd64="$(sum_of "$(asset linux amd64 tar.gz)")"
linux_arm64="$(sum_of "$(asset linux arm64 tar.gz)")"
darwin_amd64="$(sum_of "$(asset darwin amd64 tar.gz)")"
darwin_arm64="$(sum_of "$(asset darwin arm64 tar.gz)")"

win_amd64_url="$(url_of "$(asset windows amd64 zip)")"
win_arm64_url="$(url_of "$(asset windows arm64 zip)")"

mkdir -p "$root/bucket"

cat > "$root/bucket/orangeflix-dl.json" <<EOF
{
    "version": "${version}",
    "description": "${desc}",
    "homepage": "https://github.com/${repo}",
    "license": "MIT",
    "architecture": {
        "64bit": {
            "url": "${win_amd64_url}",
            "hash": "${win_amd64}"
        },
        "arm64": {
            "url": "${win_arm64_url}",
            "hash": "${win_arm64}"
        }
    },
    "bin": "orangeflix-dl.exe",
    "checkver": "github",
    "autoupdate": {
        "architecture": {
            "64bit": {
                "url": "https://github.com/${repo}/releases/download/v\$version/orangeflix-dl_v\$version_windows_amd64.zip"
            },
            "arm64": {
                "url": "https://github.com/${repo}/releases/download/v\$version/orangeflix-dl_v\$version_windows_arm64.zip"
            }
        },
        "hash": {
            "url": "https://github.com/${repo}/releases/download/v\$version/checksums.txt",
            "regex": "\$sha256\\\\s+\$basename"
        }
    }
}
EOF

cat > "$root/flake.nix" <<EOF
{
  description = "${desc}";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      version = "${version}";
      repo = "${repo}";
      systems = {
        x86_64-linux = { os = "linux"; arch = "amd64"; hash = "$(sri "$linux_amd64")"; };
        aarch64-linux = { os = "linux"; arch = "arm64"; hash = "$(sri "$linux_arm64")"; };
        x86_64-darwin = { os = "darwin"; arch = "amd64"; hash = "$(sri "$darwin_amd64")"; };
        aarch64-darwin = { os = "darwin"; arch = "arm64"; hash = "$(sri "$darwin_arm64")"; };
      };
      each = nixpkgs.lib.genAttrs (builtins.attrNames systems);
    in
    {
      packages = each (system:
        let
          pkgs = import nixpkgs { inherit system; };
          spec = systems.\${system};
          archive = "orangeflix-dl_v\${version}_\${spec.os}_\${spec.arch}.tar.gz";
        in
        {
          default = pkgs.stdenvNoCC.mkDerivation {
            pname = "orangeflix-dl";
            inherit version;
            src = pkgs.fetchurl {
              url = "https://github.com/\${repo}/releases/download/v\${version}/\${archive}";
              hash = spec.hash;
            };
            nativeBuildInputs = [ pkgs.installShellFiles ];
            dontUnpack = true;
            installPhase = ''
              runHook preInstall
              tar -xzf \$src
              install -Dm755 orangeflix-dl \$out/bin/orangeflix-dl
              installShellCompletion --cmd orangeflix-dl \\
                --bash <(\$out/bin/orangeflix-dl --completions bash) \\
                --zsh <(\$out/bin/orangeflix-dl --completions zsh) \\
                --fish <(\$out/bin/orangeflix-dl --completions fish)
              \$out/bin/orangeflix-dl --man > orangeflix-dl.1
              installManPage orangeflix-dl.1
              runHook postInstall
            '';
          };
        });
    };
}
EOF
