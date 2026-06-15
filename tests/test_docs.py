import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class DocumentationExamplesTest(unittest.TestCase):
    def test_readme_uses_existing_policy_preset(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("agent-packs policy check eng-leader default", readme)
        self.assertNotIn("agent-packs policy check eng-leader policy.json", readme)

    def test_readme_new_pack_example_does_not_collide_with_registry(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("agent-packs new pack my-custom-pack --dir registry/packs", readme)
        self.assertFalse((ROOT / "registry" / "packs" / "my-custom-pack.json").exists())
        self.assertNotIn("agent-packs new pack platform-engineer --dir registry/packs", readme)

    def test_readme_plugin_quickstart_is_preview_safe(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")

        self.assertIn("agent-packs plugins install claude-code-review", readme)
        self.assertIn("--dry-run", readme)

    def test_docs_use_correct_homebrew_tap_path(self):
        # The published tap installs as sandeshh/agent-packs/agent-packs;
        # sandeshh/tap/... is a stale, non-existent path.
        for rel in ("README.md", "docs/architecture.md"):
            text = (ROOT / rel).read_text(encoding="utf-8")
            self.assertNotIn("sandeshh/tap/agent-packs", text, rel)
            if "brew install" in text:
                self.assertIn("brew install sandeshh/agent-packs/agent-packs", text, rel)

    def test_pin_command_is_documented(self):
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        architecture = (ROOT / "docs" / "architecture.md").read_text(encoding="utf-8")
        self.assertIn("pin <pack>", readme)
        self.assertIn("agent-packs pin <pack>", architecture)

    def test_landing_page_diagram_uses_real_plugins(self):
        # The architecture diagram must reference real registry plugins, not
        # invented names, and anchor on a pack that actually bundles plugins.
        html = (ROOT / "docs" / "index.html").read_text(encoding="utf-8")
        self.assertNotIn("github-tools", html)
        self.assertNotIn("database-browser", html)
        for plugin in ("eng-leader-workflows", "github-pr-inspection"):
            self.assertTrue((ROOT / "registry" / "plugins" / plugin).is_dir(), plugin)
            self.assertIn(plugin, html, plugin)


if __name__ == "__main__":
    unittest.main()
