class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/agent-packs/cli"
  version "0.1.3"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.1.3/agent-packs_darwin_arm64.tar.gz"
      sha256 "bada72c80941f7c13144549b227387c948da126a485cc949dfc5ae3284526935"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.1.3/agent-packs_darwin_amd64.tar.gz"
      sha256 "4c7297ead7a2ef52f6c5a2f7cde5de2a752fe6980185ff39dc5a40fa97e20fca"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.1.3/agent-packs_linux_arm64.tar.gz"
      sha256 "4cbfc80c3760def0a668da32126421bd62aeab24e7b6cade9af1cf6cf49079b8"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.1.3/agent-packs_linux_amd64.tar.gz"
      sha256 "263231d032d1869134ac049e2ae9ba107c68142e1a19c665b26170df31604a28"
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
