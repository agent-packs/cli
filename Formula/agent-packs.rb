class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/sandeshh/agent-packs"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_darwin_arm64.tar.gz"
      # sha256 updated by GoReleaser on each release
      sha256 "PLACEHOLDER_DARWIN_ARM64_SHA256"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_AMD64_SHA256"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_ARM64_SHA256"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_AMD64_SHA256"
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
        agent-packs completion bash     # Shell completion

      To enable shell completion (bash):
        agent-packs completion bash > $(brew --prefix)/etc/bash_completion.d/agent-packs

      Zsh (add to ~/.zshrc):
        eval "$(agent-packs completion zsh)"
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/agent-packs version")
    assert_match "search", shell_output("#{bin}/agent-packs help 2>&1", 2)
  end
end
