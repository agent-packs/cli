class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/sandeshh/agent-packs"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_darwin_arm64.tar.gz"
      sha256 "aa0a984bfb7474991daadf992806b4cb6833640820be05dffbdb1d980596cf4a"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_darwin_amd64.tar.gz"
      sha256 "2cccec5cbe9f2c357235aa95c41d9b942049700115f9eabc5aac415f6ae0bdbe"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_linux_arm64.tar.gz"
      sha256 "70cc64a7a8d75d179dd38844abed3b147f27e9649bd47414264f93baeeb0a630"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v#{version}/agent-packs_linux_amd64.tar.gz"
      sha256 "bec591d15d1ee726a9fdb239c95aee1dc6fab81c2b668169bec78cf5f4237a2f"
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
    assert_match version.to_s, shell_output("#{bin}/agent-packs version")
  end
end
