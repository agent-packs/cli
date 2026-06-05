# Agent Packs

Curated, installable capability bundles for AI coding agents.

Agent Packs bundles public Skills, Plugins, MCP servers, commands, hooks, prompts,
and templates into ready-to-use workflow packs.

## Language Recommendation

Use Go for the production CLI.

Go is the best fit for a Homebrew-like installer because it produces small
single-file binaries, cross-compiles cleanly for macOS/Linux/Windows, starts
quickly, and does not require users to install Node, Python, or Rust first.

Recommended stack:

- CLI: Go
- Registry metadata: JSON documents validated by JSON Schema
- Pack manifests: YAML or JSON
- Web/API registry: TypeScript later, if needed
- Install scripts: POSIX shell for macOS/Linux bootstrap

This repository separates the Go CLI under `cli/` from the Agent Pack registry under `registry/`. The registry and manifest formats are intentionally language-neutral.

## Build

```sh
cd cli
go build -o bin/agent-packs ./cmd/agent-packs
```

## CLI Usage

```sh
cli/bin/agent-packs search
cli/bin/agent-packs show frontend-engineer
cli/bin/agent-packs install frontend-engineer --target ./sandbox
cli/bin/agent-packs install frontend-engineer --agent codex --only skills --dry-run
```

The prototype installer supports `--agent`, `--only`, `--dry-run`, and `--execute-plugins`. Skill capabilities with local sources are copied into the selected agent target. Remote skills and plugin commands are recorded as pending unless plugin execution is explicitly enabled.

## Specifying Plugins And Skills

Plugins and skills are declared as entries in `capabilities`. Plugin entries must include `format` and `install` metadata so an installer can resolve the marketplace/package/command. Skill entries must include `format` and `entry` so an installer can locate the `SKILL.md` file.

```json
{
  "type": "plugin",
  "name": "Anthropic Claude Code code-review plugin",
  "source": "https://github.com/anthropics/claude-plugins-official/tree/main/plugins/code-review",
  "format": "anthropic-plugin",
  "entry": ".claude-plugin/plugin.json",
  "install": {
    "method": "claude-marketplace",
    "marketplace": "claude-plugins-official",
    "package": "code-review",
    "command": "claude plugin install code-review@claude-plugins-official"
  }
}
```

```json
{
  "type": "skill",
  "name": "Microsoft Azure Agent Skills",
  "source": "https://github.com/MicrosoftDocs/Agent-Skills/tree/main/skills",
  "format": "agent-skill",
  "entry": "SKILL.md",
  "targets": [".claude/skills/", ".codex/skills/", ".github/skills/"]
}
```

## Examples

Example manifests live in `registry/schemas/examples/`:

- `minimal-pack.json`: the smallest valid pack manifest.
- `full-pack.json`: a complete manifest showing every supported capability type.
- `real-world-pack.json`: examples based on public Claude Code plugin and Agent Skills repositories.

## Tests

```sh
python3 -m unittest discover -s tests
```

## Core Concepts

- Pack: a curated bundle for a role, stack, workflow, or task.
- Skill: an instruction module, often `SKILL.md`.
- Plugin: a packaged agent extension, such as an Anthropic/Claude Code plugin.
- Tool: MCP server, shell command, API connector, or executable integration.
- Recipe: recommended combinations of packs for a larger use case.

## License

Apache-2.0
