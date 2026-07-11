import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "bin" / "agent-packs"


class InstallCommandTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        subprocess.run(
            ["go", "build", "-o", "bin/agent-packs", "./cmd/agent-packs"],
            cwd=ROOT,
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

    def write_registry_plugin(self, registry, plugin_id):
        plugin_dir = registry.parent / "plugins" / plugin_id / ".claude-plugin"
        plugin_dir.mkdir(parents=True)
        manifest = {
            "name": plugin_id,
            "displayName": "Standalone Plugin",
            "version": "0.1.0",
            "description": "A standalone plugin.",
        }
        (plugin_dir / "plugin.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")

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

            result = self.run_cli("install", "example", "--agent", "codex", "--only", "skills", "--mode", "copy", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            installed_skill = target / ".codex" / "skills" / "example-skill" / "SKILL.md"
            self.assertEqual(installed_skill.read_text(encoding="utf-8"), "# Example Skill\n")

            receipt = json.loads((target / "receipts" / "example.json").read_text(encoding="utf-8"))
            self.assertEqual(receipt["plan"]["agent"], "codex")
            self.assertEqual(receipt["plan"]["capabilities"][0]["status"], "installed")

    def test_check_gates_pins_and_drift(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            skill = temp / "skill"
            skill.mkdir()
            (skill / "SKILL.md").write_text("# Example Skill\n", encoding="utf-8")
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(
                registry,
                {
                    "id": "check-pack",
                    "name": "Check Pack",
                    "version": "0.1.0",
                    "description": "A check gate pack.",
                    "capabilities": [
                        {
                            "type": "skill",
                            "name": "check-skill",
                            "source": str(skill),
                            "format": "agent-skill",
                            "entry": "SKILL.md",
                        }
                    ],
                },
            )
            install = self.run_cli(
                "install", "check-pack", "--agent", "codex", "--only", "skills", "--mode", "copy",
                registry=registry, target=target,
            )
            self.assertEqual(install.returncode, 0, install.stderr)

            # Unpinned installs pass with a warning.
            unpinned = self.run_cli("check", registry=registry, target=target)
            self.assertEqual(unpinned.returncode, 0, unpinned.stderr)
            self.assertIn("UNPINNED", unpinned.stdout)
            self.assertIn("check passed", unpinned.stdout)

            pin = self.run_cli("pin", "check-pack", registry=registry, target=target)
            self.assertEqual(pin.returncode, 0, pin.stderr)
            pinned = self.run_cli("check", registry=registry, target=target)
            self.assertEqual(pinned.returncode, 0, pinned.stderr)
            self.assertNotIn("UNPINNED", pinned.stdout)

            # Hand-editing the installed skill fails the gate with a nonzero exit.
            installed_skill = target / ".codex" / "skills" / "check-skill" / "SKILL.md"
            installed_skill.write_text("tampered\n", encoding="utf-8")
            tampered = self.run_cli("check", registry=registry, target=target)
            self.assertNotEqual(tampered.returncode, 0)
            self.assertIn("DRIFTED", tampered.stdout)
            self.assertIn("check failed", tampered.stdout)

            tampered_json = self.run_cli("check", "--json", registry=registry, target=target)
            self.assertNotEqual(tampered_json.returncode, 0)
            report = json.loads(tampered_json.stdout)
            self.assertFalse(report["ok"])
            self.assertEqual(report["packs"][0]["id"], "check-pack")

    def test_installs_multiple_packs_and_writes_individual_receipts(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            first_skill = temp / "first-skill"
            second_skill = temp / "second-skill"
            first_skill.mkdir()
            second_skill.mkdir()
            (first_skill / "SKILL.md").write_text("# First Skill\n", encoding="utf-8")
            (second_skill / "SKILL.md").write_text("# Second Skill\n", encoding="utf-8")
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, skill_only_pack("first", "First Skill", first_skill))
            self.write_pack(registry, skill_only_pack("second", "Second Skill", second_skill))

            result = self.run_cli("install", "first", "second", "--agent", "codex", "--only", "skills", "--mode", "copy", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("==> Installing first (1/2)", result.stdout)
            self.assertIn("==> Installing second (2/2)", result.stdout)
            self.assertTrue((target / "receipts" / "first.json").exists())
            self.assertTrue((target / "receipts" / "second.json").exists())
            self.assertEqual((target / ".codex" / "skills" / "first-skill" / "SKILL.md").read_text(encoding="utf-8"), "# First Skill\n")
            self.assertEqual((target / ".codex" / "skills" / "second-skill" / "SKILL.md").read_text(encoding="utf-8"), "# Second Skill\n")

    def test_multi_pack_dry_run_does_not_write_receipts(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, skill_only_pack("first", "First Skill", temp / "first-skill"))
            self.write_pack(registry, skill_only_pack("second", "Second Skill", temp / "second-skill"))

            result = self.run_cli("install", "first", "second", "--dry-run", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Pack: first", result.stdout)
            self.assertIn("Pack: second", result.stdout)
            self.assertFalse((target / "receipts" / "first.json").exists())
            self.assertFalse((target / "receipts" / "second.json").exists())

    def test_uninstalls_multiple_packs(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            first_skill = temp / "first-skill"
            second_skill = temp / "second-skill"
            first_skill.mkdir()
            second_skill.mkdir()
            (first_skill / "SKILL.md").write_text("# First Skill\n", encoding="utf-8")
            (second_skill / "SKILL.md").write_text("# Second Skill\n", encoding="utf-8")
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, skill_only_pack("first", "First Skill", first_skill))
            self.write_pack(registry, skill_only_pack("second", "Second Skill", second_skill))
            install_result = self.run_cli("install", "first", "second", "--agent", "codex", "--only", "skills", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(install_result.returncode, 0, install_result.stderr)

            result = self.run_cli("uninstall", "first", "second", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("==> Uninstalling first (1/2)", result.stdout)
            self.assertIn("==> Uninstalling second (2/2)", result.stdout)
            self.assertFalse((target / "receipts" / "first.json").exists())
            self.assertFalse((target / "receipts" / "second.json").exists())
            self.assertFalse((target / ".codex" / "skills" / "first-skill").exists())
            self.assertFalse((target / ".codex" / "skills" / "second-skill").exists())

    def test_upgrades_multiple_packs_using_existing_receipts(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            first_skill = temp / "first-skill"
            second_skill = temp / "second-skill"
            first_skill.mkdir()
            second_skill.mkdir()
            (first_skill / "SKILL.md").write_text("# First Skill\n", encoding="utf-8")
            (second_skill / "SKILL.md").write_text("# Second Skill\n", encoding="utf-8")
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, skill_only_pack("first", "First Skill", first_skill))
            self.write_pack(registry, skill_only_pack("second", "Second Skill", second_skill))
            install_result = self.run_cli("install", "first", "second", "--agent", "codex", "--only", "skills", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(install_result.returncode, 0, install_result.stderr)

            result = self.run_cli("upgrade", "first", "second", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("==> Upgrading first (1/2)", result.stdout)
            self.assertIn("==> Upgrading second (2/2)", result.stdout)
            # Upgrade replays the capability filter recorded at install time.
            self.assertIn("Upgrading first (mode=copy, conflict=skip, scope=target, only=skills)", result.stdout)
            self.assertIn("Upgrading second (mode=copy, conflict=skip, scope=target, only=skills)", result.stdout)
            self.assertTrue((target / "receipts" / "first.json").exists())
            self.assertTrue((target / "receipts" / "second.json").exists())

    def test_plugins_are_pending_unless_execution_is_explicit(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, example_pack(temp / "missing-skill"))

            result = self.run_cli("install", "example", "--only", "plugins", "--mode", "native", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            receipt = json.loads((target / "receipts" / "example.json").read_text(encoding="utf-8"))
            capability = receipt["plan"]["capabilities"][0]
            self.assertEqual(capability["type"], "plugin")
            self.assertEqual(capability["status"], "pending")
            self.assertIn("--execute-plugins", capability["reason"])

    def test_skills_command_manages_standalone_local_skill(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            skill = temp / "skill"
            skill.mkdir()
            (skill / "SKILL.md").write_text(
                "---\nname: Standalone Skill\ndescription: A standalone skill.\n---\n# Standalone Skill\n",
                encoding="utf-8",
            )
            registry = temp / "registry" / "packs"
            target = temp / "install"
            registry.mkdir(parents=True)

            install = self.run_cli("skills", "install", str(skill), "--agent", "codex", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(install.returncode, 0, install.stderr)
            self.assertTrue((target / ".codex" / "skills" / "standalone-skill" / "SKILL.md").exists())
            self.assertTrue((target / "receipts" / "skills" / "standalone-skill.json").exists())
            self.assertFalse((target / "receipts" / "standalone-skill.json").exists())

            listed = self.run_cli("skills", "list", registry=registry, target=target)
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertIn("standalone-skill", listed.stdout)

            uninstall = self.run_cli("skills", "uninstall", "standalone-skill", registry=registry, target=target)
            self.assertEqual(uninstall.returncode, 0, uninstall.stderr)
            self.assertFalse((target / ".codex" / "skills" / "standalone-skill").exists())
            self.assertFalse((target / "receipts" / "skills" / "standalone-skill.json").exists())

    def test_plugins_command_manages_standalone_registry_plugin_with_overrides(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry" / "packs"
            target = temp / "install"
            registry.mkdir(parents=True)
            self.write_registry_plugin(registry, "standalone-plugin")
            env = os.environ.copy()
            env["AGENT_PACKS_REGISTRY"] = str(registry)
            env["AGENT_PACKS_PLUGIN_CWD"] = str(temp)

            install = subprocess.run(
                [
                    str(CLI),
                    "plugins",
                    "install",
                    "standalone-plugin",
                    "--mode",
                    "native",
                    "--execute-plugins",
                    "--method",
                    "manual",
                    "--command",
                    "printf installed > plugin-install.txt",
                    "--uninstall-command",
                    "printf cleaned > plugin-cleanup.txt",
                    "--target",
                    str(target),
                ],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
            )
            self.assertEqual(install.returncode, 0, install.stderr)
            self.assertEqual((temp / "plugin-install.txt").read_text(encoding="utf-8"), "installed")
            self.assertTrue((target / "receipts" / "plugins" / "standalone-plugin.json").exists())

            listed = self.run_cli("plugins", "list", registry=registry, target=target)
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertIn("standalone-plugin", listed.stdout)

            uninstall = subprocess.run(
                [
                    str(CLI),
                    "plugins",
                    "uninstall",
                    "standalone-plugin",
                    "--execute-plugins",
                    "--target",
                    str(target),
                ],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
            )
            self.assertEqual(uninstall.returncode, 0, uninstall.stderr)
            self.assertEqual((temp / "plugin-cleanup.txt").read_text(encoding="utf-8"), "cleaned")
            self.assertFalse((target / "receipts" / "plugins" / "standalone-plugin.json").exists())

    def test_commands_command_manages_standalone_local_command(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry" / "packs"
            target = temp / "install"
            registry.mkdir(parents=True)
            command = temp / "review-pr.json"
            command.write_text(
                json.dumps(
                    {
                        "type": "command",
                        "name": "Review PR",
                        "format": "markdown",
                        "content": "Review this pull request.",
                    },
                    indent=2,
                )
                + "\n",
                encoding="utf-8",
            )

            install = self.run_cli("commands", "install", str(command), "--agent", "codex", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(install.returncode, 0, install.stderr)
            installed_command = target / ".agent-packs" / "commands" / "review-pr.md"
            self.assertEqual(installed_command.read_text(encoding="utf-8"), "Review this pull request.")
            self.assertTrue((target / "receipts" / "commands" / "review-pr.json").exists())

            listed = self.run_cli("commands", "list", registry=registry, target=target)
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertIn("review-pr", listed.stdout)

            uninstall = self.run_cli("commands", "uninstall", "review-pr", registry=registry, target=target)
            self.assertEqual(uninstall.returncode, 0, uninstall.stderr)
            self.assertFalse(installed_command.exists())
            self.assertFalse((target / "receipts" / "commands" / "review-pr.json").exists())

    def test_memory_command_manages_standalone_local_memory(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry" / "packs"
            target = temp / "install"
            registry.mkdir(parents=True)
            memory = temp / "house-rules.json"
            memory.write_text(
                json.dumps(
                    {
                        "type": "memory",
                        "name": "House Rules",
                        "content": "Prefer tests for behavior changes.",
                    },
                    indent=2,
                )
                + "\n",
                encoding="utf-8",
            )

            install = self.run_cli("memory", "install", str(memory), "--agent", "codex", "--mode", "copy", "--project", str(target), registry=registry, target=target)
            self.assertEqual(install.returncode, 0, install.stderr)
            agents_md = target / "AGENTS.md"
            body = agents_md.read_text(encoding="utf-8")
            self.assertIn("Prefer tests for behavior changes.", body)
            self.assertIn("BEGIN agent-packs:house-rules/house-rules", body)
            self.assertTrue((target / "receipts" / "memory" / "house-rules.json").exists())

            listed = self.run_cli("memory", "list", registry=registry, target=target)
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertIn("house-rules", listed.stdout)

            uninstall = self.run_cli("memory", "uninstall", "house-rules", registry=registry, target=target)
            self.assertEqual(uninstall.returncode, 0, uninstall.stderr)
            self.assertNotIn("Prefer tests for behavior changes.", agents_md.read_text(encoding="utf-8"))
            self.assertFalse((target / "receipts" / "memory" / "house-rules.json").exists())

    def test_tools_command_manages_standalone_local_tool_descriptor(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry" / "packs"
            target = temp / "install"
            registry.mkdir(parents=True)
            tool = temp / "browser-verify.json"
            tool.write_text(
                json.dumps(
                    {
                        "type": "tool",
                        "name": "Browser Verify",
                        "format": "json",
                        "content": '{"name":"browser-verify","description":"descriptor only"}',
                    },
                    indent=2,
                )
                + "\n",
                encoding="utf-8",
            )

            install = self.run_cli("tools", "install", str(tool), "--agent", "codex", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(install.returncode, 0, install.stderr)
            installed_tool = target / ".agent-packs" / "tools" / "browser-verify.json"
            self.assertEqual(installed_tool.read_text(encoding="utf-8"), '{"name":"browser-verify","description":"descriptor only"}')
            self.assertTrue((target / "receipts" / "tools" / "browser-verify.json").exists())

            listed = self.run_cli("tools", "list", registry=registry, target=target)
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertIn("browser-verify", listed.stdout)

            uninstall = self.run_cli("tools", "uninstall", "browser-verify", registry=registry, target=target)
            self.assertEqual(uninstall.returncode, 0, uninstall.stderr)
            self.assertFalse(installed_tool.exists())
            self.assertFalse((target / "receipts" / "tools" / "browser-verify.json").exists())

    def test_install_pack_only_tools(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(
                registry,
                {
                    "id": "tool-pack",
                    "name": "Tool Pack",
                    "version": "0.1.0",
                    "description": "A tool descriptor pack.",
                    "capabilities": [
                        {"type": "command", "name": "Review PR", "content": "Review this PR."},
                        {"type": "tool", "name": "Browser Verify", "format": "json", "content": '{"name":"browser-verify"}'},
                    ],
                },
            )

            result = self.run_cli("install", "tool-pack", "--agent", "codex", "--only", "tools", "--mode", "copy", registry=registry, target=target)

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((target / ".agent-packs" / "tools" / "browser-verify.json").exists())
            self.assertFalse((target / ".agent-packs" / "commands" / "review-pr.md").exists())


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


def memory_pack(content, name="House Rules"):
    return {
        "id": "mem-pack",
        "name": "Memory Pack",
        "version": "0.1.0",
        "description": "A memory test pack.",
        "capabilities": [
            {"type": "memory", "name": name, "content": content},
        ],
    }


def settings_pack(content):
    return {
        "id": "set-pack",
        "name": "Settings Pack",
        "version": "0.1.0",
        "description": "A settings test pack.",
        "capabilities": [
            {"type": "settings", "name": "model", "content": content},
        ],
    }


class MergeCapabilityTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        subprocess.run(
            ["go", "build", "-o", "bin/agent-packs", "./cmd/agent-packs"],
            cwd=ROOT,
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
        (registry / f"{pack['id']}.json").write_text(json.dumps(pack, indent=2) + "\n", encoding="utf-8")

    def test_memory_install_upgrade_uninstall_lifecycle(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            # Default scope is global, which for Claude is .claude/CLAUDE.md.
            claude_md = target / ".claude" / "CLAUDE.md"
            claude_md.parent.mkdir(parents=True)
            claude_md.write_text("# My project notes\n\nKeep me.\n", encoding="utf-8")

            self.write_pack(registry, memory_pack("Use tabs, not spaces."))
            result = self.run_cli("install", "mem-pack", "--agent", "claude", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(result.returncode, 0, result.stderr)
            body = claude_md.read_text(encoding="utf-8")
            self.assertIn("Keep me.", body)
            self.assertIn("Use tabs, not spaces.", body)
            self.assertEqual(body.count("BEGIN agent-packs:mem-pack/house-rules"), 1)

            # Upgrade with changed content -> single block, new body.
            self.write_pack(registry, memory_pack("Use spaces, not tabs."))
            up = self.run_cli("upgrade", "mem-pack", registry=registry, target=target)
            self.assertEqual(up.returncode, 0, up.stderr)
            body = claude_md.read_text(encoding="utf-8")
            self.assertEqual(body.count("BEGIN agent-packs:mem-pack/house-rules"), 1)
            self.assertIn("Use spaces, not tabs.", body)
            self.assertNotIn("Use tabs, not spaces.", body)

            # Uninstall removes only our block, restores the file.
            un = self.run_cli("uninstall", "mem-pack", registry=registry, target=target)
            self.assertEqual(un.returncode, 0, un.stderr)
            self.assertEqual(claude_md.read_text(encoding="utf-8"), "# My project notes\n\nKeep me.\n")

    def test_settings_merge_preserves_user_keys(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            settings = target / ".claude" / "settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text(json.dumps({"theme": "dark"}) + "\n", encoding="utf-8")

            self.write_pack(registry, settings_pack('{"model":"opus"}'))
            result = self.run_cli("install", "set-pack", "--agent", "claude", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(result.returncode, 0, result.stderr)
            merged = json.loads(settings.read_text(encoding="utf-8"))
            self.assertEqual(merged["theme"], "dark")
            self.assertEqual(merged["model"], "opus")

            un = self.run_cli("uninstall", "set-pack", registry=registry, target=target)
            self.assertEqual(un.returncode, 0, un.stderr)
            after = json.loads(settings.read_text(encoding="utf-8"))
            self.assertEqual(after, {"theme": "dark"})

    def test_unsupported_agent_type_pair_skips(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, settings_pack('{"model":"opus"}'))
            # cursor has no settings destination -> skip+unsupported, no file written.
            result = self.run_cli("install", "set-pack", "--agent", "cursor", "--mode", "copy", registry=registry, target=target)
            self.assertEqual(result.returncode, 0, result.stderr)
            receipt = json.loads((target / "receipts" / "set-pack.json").read_text(encoding="utf-8"))
            self.assertEqual(receipt["plan"]["capabilities"][0]["status"], "unsupported")

    def test_reference_mode_does_not_write_user_file(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp = Path(temp_dir)
            registry = temp / "registry"
            target = temp / "install"
            registry.mkdir()
            self.write_pack(registry, memory_pack("Use tabs."))
            # Default mode is reference -> record only, never touch CLAUDE.md.
            result = self.run_cli("install", "mem-pack", "--agent", "claude", registry=registry, target=target)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((target / "CLAUDE.md").exists())
            receipt = json.loads((target / "receipts" / "mem-pack.json").read_text(encoding="utf-8"))
            self.assertEqual(receipt["plan"]["capabilities"][0]["status"], "recorded")


def skill_only_pack(pack_id, skill_name, skill_source):
    return {
        "id": pack_id,
        "name": f"{skill_name} Pack",
        "version": "0.1.0",
        "description": "A test pack.",
        "capabilities": [
            {
                "type": "skill",
                "name": skill_name,
                "source": str(skill_source),
                "format": "agent-skill",
                "entry": "SKILL.md",
            }
        ],
    }


if __name__ == "__main__":
    unittest.main()
