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


if __name__ == "__main__":
    unittest.main()
