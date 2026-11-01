class OrangeflixDl < Formula
  REPO = "revunix/orangeflix-dl".freeze
  # Rückfall, wenn das neueste Release nicht abgefragt werden kann.
  PINNED_TAG = "v1.1.1".freeze
  PINNED_SUMS = {
    ["darwin", "arm64"] => "51e49368ba264715fe4392fc37e00b101ac173db7a542071f2058e44332a7e22",
    ["darwin", "amd64"] => "695065f7d90aed0fd84d02e3d07126b7275ebd9a52cb8ef0effe87a5e942f7f1",
    ["linux", "arm64"]  => "c3d9469f8027df3692a07dc0de47a0daa336db1370efd4d8042432a751cdffad",
    ["linux", "amd64"]  => "7713dec845f0ca83e88f0abaf056b4c29117a5f9682b494e53d56f6e75a009e3",
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
