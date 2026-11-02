class OrangeflixDl < Formula
  REPO = "revunix/orangeflix-dl".freeze
  # Rückfall, wenn das neueste Release nicht abgefragt werden kann.
  PINNED_TAG = "v1.1.3".freeze
  PINNED_SUMS = {
    ["darwin", "arm64"] => "644269781e1d8852b99efd1b3c5ad55a21d60b809c84013aca7f8819160f39a3",
    ["darwin", "amd64"] => "cc78d77f00ac12ccec7893e4faf21fcab15508b725918f2699ae4e9dc7045a42",
    ["linux", "arm64"]  => "bb68ef4320a7105603e6360767c0e4f751f5225e30a2b4f92e6a70778d1ba208",
    ["linux", "amd64"]  => "460925ab5070f35e30d800f2b72072ee3652f0aa7fd0099df1006e200fa1c293",
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
