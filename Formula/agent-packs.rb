class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/agent-packs/cli"
  version "0.2.1"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.2.1/agent-packs_darwin_arm64.tar.gz"
      sha256 "67c5523ab5843ce2474a47cb50cd810a6921899643d09f97b8bc72280236e0d7"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.2.1/agent-packs_darwin_amd64.tar.gz"
      sha256 "a34e0a10aa3f8335c368f3ef469406325679e83344bb55ec889346aadf6e9441"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.2.1/agent-packs_linux_arm64.tar.gz"
      sha256 "6d7a341e0d3df8ef256754cbeeb765eafb5ac2b65c47973d64ef2a1c00452da7"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.2.1/agent-packs_linux_amd64.tar.gz"
      sha256 "3c55118391e295727b74795f8c3bc461092d2df71fe6c448563c9fdb3d0753eb"
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
