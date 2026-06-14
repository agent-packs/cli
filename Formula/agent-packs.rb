class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/sandeshh/agent-packs"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_darwin_arm64.tar.gz"
      sha256 "e4db9b808f223e42de72f7e01a35ed6b2d52a953d133b74ac28955cee9438c27"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_darwin_amd64.tar.gz"
      sha256 "25c46e2a188ca0b529669539b5c875f240cbbd5757df813c918bbed6f62a017a"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_linux_arm64.tar.gz"
      sha256 "0f929cd105be3eedac5ebe667ea4aba71bbf85f72931e58d975fea955122170a"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_linux_amd64.tar.gz"
      sha256 "4d68dabfa3e73b9573bc2683e9da51c5887881906077edd4ef22921484b729c4"
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
        eval "$(agent-packs completion zsh)"
    EOS
  end

  test do
    assert_match version.to_s, shell_output("\#{bin}/agent-packs version")
  end
end
