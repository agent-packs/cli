class AgentPacks < Formula
  desc "Homebrew for AI agent skills and plugins — install curated packs into Claude Code, Cursor, Codex, and more"
  homepage "https://github.com/sandeshh/agent-packs"
  version "0.1.2"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.2/agent-packs_darwin_arm64.tar.gz"
      sha256 "a06a1c5c26efb06a2c109e025c990453d32f5168b5a5ac9d6e9207ac1b45c77d"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.2/agent-packs_darwin_amd64.tar.gz"
      sha256 "743377472152c4f3bc57953fdccffb328b0344ba9c6573c39d1a471d8fd38cfe"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.2/agent-packs_linux_arm64.tar.gz"
      sha256 "ef29d7b49babc53fa5d6daf3daa46dea1dcec6826ad66fb39d7bb7d641fd436c"
    else
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.2/agent-packs_linux_amd64.tar.gz"
      sha256 "ec1d235dcc7a12bcda68ee49f113eb27bb2ee4b0fe1d53104341ccb79abe946c"
    end
  end

  def install
    bin.install "agent-packs"
    (share/"agent-packs").install "registry"
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
