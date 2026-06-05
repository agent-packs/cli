class AgentPacks < Formula
  desc "Curated, installable capability bundles for AI coding agents"
  homepage "https://github.com/sandeshh/agent-packs"
  version "0.1.0"
  license "Apache-2.0"

  on_macos do
    on_intel do
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_darwin_amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_arm do
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_darwin_arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_linux_amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
    on_arm do
      url "https://github.com/sandeshh/agent-packs/releases/download/v0.1.0/agent-packs_linux_arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  def install
    bin.install "agent-packs"
    pkgshare.install "skills" if Dir.exist?("skills")
  end

  def caveats
    <<~EOS
      Agent Packs includes a Codex helper skill at:
        #{pkgshare}/skills/agent-packs

      To install the skill globally for Codex:
        mkdir -p ~/.codex/skills
        cp -R #{pkgshare}/skills/agent-packs ~/.codex/skills/
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/agent-packs version")
    assert_path_exists pkgshare/"skills/agent-packs/SKILL.md"
  end
end
