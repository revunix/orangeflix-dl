#!/usr/bin/env bash
# Schreibt Formula/orangeflix-dl.rb für ein Release.
#
#   scripts/update-formula.sh v1.2.3 dist/checksums.txt
#
# Die Formel fragt beim Laden selbst das neueste Release ab (Version, URLs und
# Prüfsummen aus dessen checksums.txt). Die hier eingetragenen Werte sind der
# Rückfall, wenn GitHub nicht erreichbar ist. Der Release-Workflow ruft das
# Skript nach dem Veröffentlichen auf.
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <tag> <checksums.txt>" >&2
  exit 2
fi

tag="$1"
checksums="$2"
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

darwin_arm64="$(sum darwin arm64)"
darwin_amd64="$(sum darwin amd64)"
linux_arm64="$(sum linux arm64)"
linux_amd64="$(sum linux amd64)"

mkdir -p "$root/Formula"
cat > "$root/Formula/orangeflix-dl.rb" <<'EOF'
class OrangeflixDl < Formula
  REPO = "@REPO@".freeze
  # Rückfall, wenn das neueste Release nicht abgefragt werden kann.
  PINNED_TAG = "@TAG@".freeze
  PINNED_SUMS = {
    ["darwin", "arm64"] => "@DARWIN_ARM64@",
    ["darwin", "amd64"] => "@DARWIN_AMD64@",
    ["linux", "arm64"]  => "@LINUX_ARM64@",
    ["linux", "amd64"]  => "@LINUX_AMD64@",
  }.freeze

  # Liest die checksums.txt des neuesten Releases. Die Prüfsummen bleiben
  # damit erhalten: die Archive werden gegen die Summen desselben Releases
  # geprüft. Ohne Netz oder bei unvollständiger Antwort gilt der Rückfall.
  def self.latest_release
    require "net/http"
    uri = URI("https://github.com/#{REPO}/releases/latest/download/checksums.txt")
    body = nil
    5.times do
      response = Net::HTTP.start(uri.host, uri.port, use_ssl: true, open_timeout: 3, read_timeout: 5) do |http|
        http.get(uri.request_uri)
      end
      if response.is_a?(Net::HTTPRedirection)
        uri = URI(response["location"])
        next
      end
      body = response.body if response.is_a?(Net::HTTPSuccess)
      break
    end
    return nil if body.nil?

    tags = []
    sums = {}
    body.each_line do |line|
      digest, name = line.split
      match = name&.match(/\Aorangeflix-dl_(v\d+(?:\.\d+)*)_(darwin|linux)_(amd64|arm64)\.tar\.gz\z/)
      next unless match && digest&.match?(/\A\h{64}\z/)

      tags << match[1]
      sums[[match[2], match[3]]] = digest
    end
    return nil if tags.uniq.length != 1 || sums.length != PINNED_SUMS.length

    { tag: tags.first, sums: sums }
  rescue StandardError
    nil
  end

  def self.release
    @release ||= latest_release || { tag: PINNED_TAG, sums: PINNED_SUMS }
  end

  def self.archive(os, arch)
    tag = release[:tag]
    "https://github.com/#{REPO}/releases/download/#{tag}/orangeflix-dl_#{tag}_#{os}_#{arch}.tar.gz"
  end

  desc "Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen (TUI und CLI)"
  homepage "https://github.com/#{REPO}"
  version release[:tag].delete_prefix("v")
  license "MIT"

  on_macos do
    on_arm do
      url archive("darwin", "arm64")
      sha256 release[:sums][["darwin", "arm64"]]
    end
    on_intel do
      url archive("darwin", "amd64")
      sha256 release[:sums][["darwin", "amd64"]]
    end
  end

  on_linux do
    on_arm do
      url archive("linux", "arm64")
      sha256 release[:sums][["linux", "arm64"]]
    end
    on_intel do
      url archive("linux", "amd64")
      sha256 release[:sums][["linux", "amd64"]]
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

sed -i.bak \
  -e "s|@REPO@|${repo}|g" \
  -e "s|@TAG@|${tag}|g" \
  -e "s|@DARWIN_ARM64@|${darwin_arm64}|g" \
  -e "s|@DARWIN_AMD64@|${darwin_amd64}|g" \
  -e "s|@LINUX_ARM64@|${linux_arm64}|g" \
  -e "s|@LINUX_AMD64@|${linux_amd64}|g" \
  "$root/Formula/orangeflix-dl.rb"
rm -f "$root/Formula/orangeflix-dl.rb.bak"
