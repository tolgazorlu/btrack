class Btrack < Formula
  desc "AI-native CLI time tracker with git-style workflow"
  homepage "https://github.com/tolgazorlu/btrack"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/tolgazorlu/btrack/releases/download/v0.1.0/btrack-darwin-arm64.tar.gz"
      sha256 "ae059c7d8796856806f075a61b6872bb22e2012dfca55d4278363655d79b92e4"
    else
      url "https://github.com/tolgazorlu/btrack/releases/download/v0.1.0/btrack-darwin-amd64.tar.gz"
      sha256 "557843b549a0ff8bd6b0683006f2d04ea6310e25c424556b5b9b970538c8c264"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/tolgazorlu/btrack/releases/download/v0.1.0/btrack-linux-arm64.tar.gz"
      sha256 "9959d08f31d3a13e6a4e08117be73f94c48b00d872fc75b57682aed50a217e05"
    else
      url "https://github.com/tolgazorlu/btrack/releases/download/v0.1.0/btrack-linux-amd64.tar.gz"
      sha256 "d203a0df99a20b213d60ef42321be8a6acb59c1fcc9ff68461ad40f9e0df3afa"
    end
  end

  def install
    bin.install "btrack"
  end

  test do
    system "#{bin}/btrack", "--version"
  end
end
