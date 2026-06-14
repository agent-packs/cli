#!/usr/bin/env bash
# Generates Formula/agent-packs.rb for the Homebrew tap.
# Usage: gen-formula.sh <version> <darwin_arm64_sha> <darwin_amd64_sha> <linux_arm64_sha> <linux_amd64_sha>
set -euo pipefail

VERSION="${1#v}"   # strip leading v
SHA_DARWIN_ARM64="$2"
SHA_DARWIN_AMD64="$3"
SHA_LINUX_ARM64="$4"
SHA_LINUX_AMD64="$5"
OUTPUT="${6:-Formula/agent-packs.rb}"

mkdir -p "$(dirname "$OUTPUT")"

cat > "$OUTPUT" <<RUBY
class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/sandeshh/agent-packs"
  version "${VERSION}"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v${VERSION}/agent-packs_darwin_arm64.tar.gz"
      sha256 "${SHA_DARWIN_ARM64}"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v${VERSION}/agent-packs_darwin_amd64.tar.gz"
      sha256 "${SHA_DARWIN_AMD64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v${VERSION}/agent-packs_linux_arm64.tar.gz"
      sha256 "${SHA_LINUX_ARM64}"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v${VERSION}/agent-packs_linux_amd64.tar.gz"
      sha256 "${SHA_LINUX_AMD64}"
    end
  end

  def install
    bin.install "agent-packs"
  end

  def caveats
    <<~EOS
      Get started:
        agent-packs search              # Browse the registry
        agent-packs install backend-engineer --agent claude
        agent-packs doctor              # Check your environment

      Shell completion (add to ~/.zshrc):
        eval "\$(agent-packs completion zsh)"
    EOS
  end

  test do
    assert_match version.to_s, shell_output("\#{bin}/agent-packs version")
  end
end
RUBY

echo "Written: $OUTPUT"
