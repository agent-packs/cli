import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "bin" / "agent-packs"


class InitDetectionTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        subprocess.run(
            ["go", "build", "-o", "bin/agent-packs", "./cmd/agent-packs"],
            cwd=ROOT,
            check=True,
            text=True,
            capture_output=True,
        )

    def run_init(self, project, registry, *extra):
        env = os.environ.copy()
        env["AGENT_PACKS_REGISTRY"] = str(registry)
        return subprocess.run(
            [str(CLI), "init", str(project), "--registry", str(registry), *extra],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
        )

    def write_pack(self, registry, pack_id, tags):
        pack = {
            "id": pack_id,
            "name": pack_id,
            "version": "1.0.0",
            "description": "A test pack.",
            "tags": tags,
        }
        (registry / f"{pack_id}.json").write_text(json.dumps(pack) + "\n", encoding="utf-8")

    def test_init_detects_agent_and_recommends_packs(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "packs"
            project = temp / "proj"
            registry.mkdir()
            (project / ".claude").mkdir(parents=True)
            (project / "go.mod").write_text("module demo\n", encoding="utf-8")
            self.write_pack(registry, "backend-engineer", ["go", "backend"])
            self.write_pack(registry, "frontend-engineer", ["javascript", "frontend"])

            result = self.run_init(project, registry)
            self.assertEqual(result.returncode, 0, result.stderr)

            config = (project / ".agent-packs.yaml").read_text(encoding="utf-8")
            self.assertIn("agent: claude", config)
            self.assertIn("backend-engineer", config)
            self.assertNotIn("frontend-engineer", config)

    def test_no_detect_writes_flag_defaults_only(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "packs"
            project = temp / "proj"
            registry.mkdir()
            (project / ".claude").mkdir(parents=True)
            (project / "go.mod").write_text("module demo\n", encoding="utf-8")
            self.write_pack(registry, "backend-engineer", ["go", "backend"])

            result = self.run_init(project, registry, "--no-detect")
            self.assertEqual(result.returncode, 0, result.stderr)

            config = (project / ".agent-packs.yaml").read_text(encoding="utf-8")
            self.assertNotIn("backend-engineer", config)
            self.assertNotIn("packs:", config)

    def test_explicit_agent_overrides_detection(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "packs"
            project = temp / "proj"
            registry.mkdir()
            (project / ".claude").mkdir(parents=True)

            result = self.run_init(project, registry, "--agent", "codex")
            self.assertEqual(result.returncode, 0, result.stderr)
            config = (project / ".agent-packs.yaml").read_text(encoding="utf-8")
            self.assertIn("agent: codex", config)


if __name__ == "__main__":
    unittest.main()
