import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class DocumentationExamplesTest(unittest.TestCase):
    def test_readme_uses_existing_policy_preset(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("agent-packs policy check eng-leader default", readme)
        self.assertNotIn("agent-packs policy check eng-leader policy.json", readme)

    def test_readme_new_pack_example_targets_registry_checkout(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")

        # Registry authoring happens in a clone of agent-packs/registry, where
        # packs live at packs/ (not registry/packs/).
        self.assertIn("agent-packs new pack my-pack --dir packs", readme)
        self.assertNotIn("--dir registry/packs", readme)

    def test_readme_plugin_quickstart_is_preview_safe(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("agent-packs plugins install claude-code-review", readme)
        self.assertIn("--dry-run", readme)

    def test_docs_use_correct_homebrew_tap_path(self):
        # The org tap installs as agent-packs/tap/agent-packs; the old
        # sandeshh/* paths are stale after the org move.
        for rel in ("README.md", "docs/architecture.md"):
            text = (ROOT / rel).read_text(encoding="utf-8")
            self.assertNotIn("sandeshh/agent-packs", text, rel)
            if "brew install" in text:
                self.assertIn("brew install agent-packs/tap/agent-packs", text, rel)

    def test_pin_command_is_documented(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        architecture = (ROOT / "docs" / "architecture.md").read_text(encoding="utf-8")
        self.assertIn("pin <pack>", readme)
        self.assertIn("agent-packs pin <pack>", architecture)

    def test_landing_page_diagram_uses_real_plugins(self):
        # The architecture diagram must reference real registry plugins (which
        # live in the agent-packs/registry repo), not invented names.
        html = (ROOT / "docs" / "index.html").read_text(encoding="utf-8")
        self.assertNotIn("github-tools", html)
        self.assertNotIn("database-browser", html)
        for plugin in ("eng-leader-workflows", "github-pr-inspection"):
            self.assertIn(plugin, html, plugin)

    def test_docs_point_at_split_repos(self):
        # After the org move + CLI/registry split, docs must reference the new
        # repos and not the old monorepo path.
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        self.assertIn("github.com/agent-packs/registry", readme)
        self.assertNotIn("sandeshh/agent-packs", readme)


if __name__ == "__main__":
    unittest.main()
