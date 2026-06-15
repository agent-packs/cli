class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/agent-packs/cli"
  version "0.2.0"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.2.0/agent-packs_darwin_arm64.tar.gz"
      sha256 "99c5982e1bf5f264a4dffe662ca28bb8002ab6481c0c591c66b1d9ddd7b4c6f1"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.2.0/agent-packs_darwin_amd64.tar.gz"
      sha256 "f8418ee494d6323dbd70edda918cc9f9366beffbafff1b44081c958f847e664a"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/agent-packs/cli/releases/download/v0.2.0/agent-packs_linux_arm64.tar.gz"
      sha256 "f56af607fcdffc4675799b0ff880d0e921ef7bb62ae6bcdc2030f92fc664bf25"
    else
      url "https://github.com/agent-packs/cli/releases/download/v0.2.0/agent-packs_linux_amd64.tar.gz"
      sha256 "8ee5717eba02040df10aa9c3f0e2ef9b9c741c557be0fdfef42a2ff91c3852a7"
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
