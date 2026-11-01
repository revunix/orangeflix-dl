class OrangeflixDl < Formula
  REPO = "revunix/orangeflix-dl".freeze
  # Rückfall, wenn das neueste Release nicht abgefragt werden kann.
  PINNED_TAG = "v1.1.2".freeze
  PINNED_SUMS = {
    ["darwin", "arm64"] => "1f6489d550bf38bcb5b045acc3d2a9b2bca2f61fbae82a00ad7d4901295d276d",
    ["darwin", "amd64"] => "559ff85a7bce4ba0ef3a59f93b0974727a464b81aa32d9fa0c536c38bee64061",
    ["linux", "arm64"]  => "7b358a2b0f8d914de1186e16987ea37887d96d8bb7aba09b3cb9d5905e0c5d82",
    ["linux", "amd64"]  => "e1681c292f7fc05dd1c2b093a99690ac734dc980de615910a0a83b0eb1dacb97",
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
