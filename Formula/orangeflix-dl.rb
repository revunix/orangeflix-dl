class OrangeflixDl < Formula
  desc "Hilfswerkzeug zur lokalen Archivierung der Orangeflix-Folgen (TUI und CLI)"
  homepage "https://github.com/revunix/orangeflix-dl"
  version "1.1.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/revunix/orangeflix-dl/releases/download/v1.1.1/orangeflix-dl_v1.1.1_darwin_arm64.tar.gz"
      sha256 "51e49368ba264715fe4392fc37e00b101ac173db7a542071f2058e44332a7e22"
    end
    on_intel do
      url "https://github.com/revunix/orangeflix-dl/releases/download/v1.1.1/orangeflix-dl_v1.1.1_darwin_amd64.tar.gz"
      sha256 "695065f7d90aed0fd84d02e3d07126b7275ebd9a52cb8ef0effe87a5e942f7f1"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/revunix/orangeflix-dl/releases/download/v1.1.1/orangeflix-dl_v1.1.1_linux_arm64.tar.gz"
      sha256 "c3d9469f8027df3692a07dc0de47a0daa336db1370efd4d8042432a751cdffad"
    end
    on_intel do
      url "https://github.com/revunix/orangeflix-dl/releases/download/v1.1.1/orangeflix-dl_v1.1.1_linux_amd64.tar.gz"
      sha256 "7713dec845f0ca83e88f0abaf056b4c29117a5f9682b494e53d56f6e75a009e3"
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
