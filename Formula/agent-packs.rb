class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/agent-packs/cli"
  version "0.3.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.3.0/agent-packs_darwin_arm64.tar.gz"
      sha256 "e1f6ad53ca31d44958a7d87577fbe4760d4179d5c51edac4451b7d1c43623882"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.3.0/agent-packs_darwin_amd64.tar.gz"
      sha256 "5b5bb3c7166221f56a59505df9e405915e5b76b91a1e37421ee460de3f42a872"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.3.0/agent-packs_linux_arm64.tar.gz"
      sha256 "56c06e41f459dbfdc815e0bfb30fd5bbaad80b7fece1b28082e54955f5054487"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.3.0/agent-packs_linux_amd64.tar.gz"
      sha256 "5b54568d2ccbf01061dd86e624f8f4ff48330914d19f95be447cd1ee5117d36c"
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
