class Btrack < Formula
  desc "AI-native CLI time tracker with git-style workflow"
  homepage "https://github.com/tolgazorlu/btrack"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/tolgazorlu/btrack/releases/download/v#{version}/btrack-darwin-arm64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_ARM64"
    else
      url "https://github.com/tolgazorlu/btrack/releases/download/v#{version}/btrack-darwin-amd64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/tolgazorlu/btrack/releases/download/v#{version}/btrack-linux-arm64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_LINUX_ARM64"
    else
      url "https://github.com/tolgazorlu/btrack/releases/download/v#{version}/btrack-linux-amd64"
      sha256 "REPLACE_WITH_ACTUAL_SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install Dir["btrack*"].first => "btrack"
  end

  def post_install
    (var/"btrack").mkpath
  end

  test do
    system "#{bin}/btrack", "--version"
  end
end
