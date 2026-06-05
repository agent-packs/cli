import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
BUNDLED_SKILL = ROOT / "skills" / "agent-packs"


class BundledSkillTest(unittest.TestCase):
    def test_agent_packs_skill_has_required_metadata(self):
        skill = BUNDLED_SKILL / "SKILL.md"
        self.assertTrue(skill.exists())

        text = skill.read_text(encoding="utf-8")
        self.assertTrue(text.startswith("---\n"))
        self.assertIn("name: agent-packs", text)
        self.assertIn("description:", text)
        self.assertIn("## Core CLI Workflows", text)

    def test_openai_metadata_has_default_prompt(self):
        metadata = BUNDLED_SKILL / "agents" / "openai.yaml"
        self.assertTrue(metadata.exists())

        text = metadata.read_text(encoding="utf-8")
        self.assertIn('display_name: "Agent Packs"', text)
        self.assertIn('default_prompt: "Use $agent-packs', text)

    def test_bootstrap_installer_installs_bundled_skill(self):
        installer = (ROOT / "install.sh").read_text(encoding="utf-8")

        self.assertIn("AGENT_PACKS_INSTALL_SKILL", installer)
        self.assertIn("AGENT_PACKS_SKILL_DIR", installer)
        self.assertIn("skills/agent-packs", installer)

    def test_release_archive_includes_bundled_skill(self):
        release = (ROOT / ".github" / "workflows" / "release.yml").read_text(encoding="utf-8")

        self.assertIn("cp -R ../skills", release)
        self.assertIn("agent-packs skills", release)


if __name__ == "__main__":
    unittest.main()
