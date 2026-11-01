#!/usr/bin/env bash
# Schreibt Formula/orangeflix-dl.rb für ein Release.
#
#   scripts/update-formula.sh v1.2.3 dist/checksums.txt
#
# Die Prüfsummen kommen aus der checksums.txt des Releases; der Release-
# Workflow ruft das Skript nach dem Veröffentlichen auf.
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

sum() {
  local file="orangeflix-dl_${tag}_$1_$2.tar.gz"
  local value
  value="$(awk -v f="$file" '$2 == f { print $1 }' "$checksums")"
  if [ -z "$value" ]; then
    echo "no checksum for $file in $checksums" >&2
    exit 1
  fi
  printf '%s' "$value"
}

url() {
  printf 'https://github.com/%s/releases/download/%s/orangeflix-dl_%s_%s_%s.tar.gz' "$repo" "$tag" "$tag" "$1" "$2"
}

mkdir -p "$root/Formula"
cat > "$root/Formula/orangeflix-dl.rb" <<EOF
class OrangeflixDl < Formula
  desc "Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen (TUI und CLI)"
  homepage "https://github.com/${repo}"
  version "${version}"
  license "MIT"

  on_macos do
    on_arm do
      url "$(url darwin arm64)"
      sha256 "$(sum darwin arm64)"
    end
    on_intel do
      url "$(url darwin amd64)"
      sha256 "$(sum darwin amd64)"
    end
  end

  on_linux do
    on_arm do
      url "$(url linux arm64)"
      sha256 "$(sum linux arm64)"
    end
    on_intel do
      url "$(url linux amd64)"
      sha256 "$(sum linux amd64)"
    end
  end

  def install
    bin.install "orangeflix-dl"

    (bash_completion/"orangeflix-dl").write Utils.safe_popen_read(bin/"orangeflix-dl", "--completions", "bash")
    (zsh_completion/"_orangeflix-dl").write Utils.safe_popen_read(bin/"orangeflix-dl", "--completions", "zsh")
    (fish_completion/"orangeflix-dl.fish").write Utils.safe_popen_read(bin/"orangeflix-dl", "--completions", "fish")
    (man1/"orangeflix-dl.1").write Utils.safe_popen_read(bin/"orangeflix-dl", "--man")
  end

  def caveats
    <<~EOS
      Zum Ansehen statt Herunterladen wird mpv gebraucht:
        brew install mpv
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/orangeflix-dl --version")
  end
end
EOF
