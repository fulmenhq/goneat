class Goneat < Formula
  desc "Single CLI to make codebases neat (format, lint, security)"
  homepage "https://github.com/fulmenhq/goneat"
  version "0.2.0-rc.8"

  on_macos do
    on_arm do
      url "https://github.com/fulmenhq/goneat/releases/download/v#{version}/goneat_#{version}_darwin_arm64.tar.gz"
      sha256 "1664c97314171a5ae72a41db79fca5705abcef63467732a229cf9f3a41db61c7"
    end
    on_intel do
      url "https://github.com/fulmenhq/goneat/releases/download/v#{version}/goneat_#{version}_darwin_amd64.tar.gz"
      sha256 "b5cd358086dae4f5ad3e33175bfb668c19213782bd45b79a0cea4ea15bc83702"
    end
  end

  on_linux do
    on_arm do
      if Hardware::CPU.is_64_bit?
        url "https://github.com/fulmenhq/goneat/releases/download/v#{version}/goneat_#{version}_linux_arm64.tar.gz"
        sha256 "4bc761a16d0e0670ebf98b8165b4397a284d7aab42740e32c0639eb5bf7ea240"
      end
    end
    on_intel do
      url "https://github.com/fulmenhq/goneat/releases/download/v#{version}/goneat_#{version}_linux_amd64.tar.gz"
      sha256 "dfb51c84d5a7c0fe6e83c9b45c1378c7c75a89ba06c8175a930d17d22bdd8d57"
    end
  end

  def install
    bin.install "goneat"
  end

  test do
    assert_match "goneat", shell_output("#{bin}/goneat version")
  end
end
