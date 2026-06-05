import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
CLI = ROOT / "dev" / "bin" / "agent-packs"


class InstallCommandTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        subprocess.run(
            ["go", "build", "-o", "bin/agent-packs", "./cmd/agent-packs"],
            cwd=ROOT / "dev",
            check=True,
            text=True,
            capture_output=True,
        )

    def run_cli(self, *args, registry, target):
        env = os.environ.copy()
        env["AGENT_PACKS_REGISTRY"] = str(registry)
        return subprocess.run(
            [str(CLI), *args, "--target", str(target)],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
        )

    def write_pack(self, registry, pack):
        path = registry / f"{pack['id']}.json"
        path.write_text(json.dumps(pack, indent=2) + "\n", encoding="utf-8")
        return path

    def test_dry_run_prints_skill_and_plugin_plan_without_writing_receipt(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, example_pack(temp / "skill"))

            result = self.run_cli("install", "example", "--dry-run", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Pack: example", result.stdout)
            self.assertIn("plugin: Example plugin", result.stdout)
            self.assertIn("command: echo install-plugin", result.stdout)
            self.assertFalse((target / "receipts" / "example.json").exists())

    def test_installs_local_skill_and_writes_receipt(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            skill = temp / "skill"
            skill.mkdir()
            (skill / "SKILL.md").write_text("# Example Skill\n", encoding="utf-8")
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, example_pack(skill))

            result = self.run_cli("install", "example", "--agent", "codex", "--only", "skills", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            installed_skill = target / ".codex" / "skills" / "example-skill" / "SKILL.md"
            self.assertEqual(installed_skill.read_text(encoding="utf-8"), "# Example Skill\n")

            receipt = json.loads((target / "receipts" / "example.json").read_text(encoding="utf-8"))
            self.assertEqual(receipt["plan"]["agent"], "codex")
            self.assertEqual(receipt["plan"]["capabilities"][0]["status"], "installed")

    def test_plugins_are_pending_unless_execution_is_explicit(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, example_pack(temp / "missing-skill"))

            result = self.run_cli("install", "example", "--only", "plugins", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            receipt = json.loads((target / "receipts" / "example.json").read_text(encoding="utf-8"))
            capability = receipt["plan"]["capabilities"][0]
            self.assertEqual(capability["type"], "plugin")
            self.assertEqual(capability["status"], "pending")
            self.assertIn("--execute-plugins", capability["reason"])


def example_pack(skill_source):
    return {
        "id": "example",
        "name": "Example Pack",
        "version": "0.1.0",
        "description": "A test pack.",
        "capabilities": [
            {
                "type": "skill",
                "name": "Example Skill",
                "source": str(skill_source),
                "format": "agent-skill",
                "entry": "SKILL.md",
            },
            {
                "type": "plugin",
                "name": "Example plugin",
                "source": "https://example.com/plugin",
                "format": "anthropic-plugin",
                "entry": ".claude-plugin/plugin.json",
                "install": {
                    "method": "manual",
                    "package": "example-plugin",
                    "command": "echo install-plugin",
                },
            },
        ],
    }


if __name__ == "__main__":
    unittest.main()
