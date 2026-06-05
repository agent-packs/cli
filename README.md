# Agent Packs

Curated, installable capability bundles for AI coding agents.

Agent Packs bundles public Skills, Plugins, MCP servers, commands, hooks, prompts,
templates, and composed packs into ready-to-use workflow packs.

## Repository Layout

- `cli/`: Go CLI module and source.
- `registry/`: pack manifests, JSON Schema, and example manifests.
- `docs/`: architecture notes.
- `tests/`: Python schema and CLI integration tests.

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

Additional commands:

```sh
cli/bin/agent-packs registry add local /path/to/agent-packs
cli/bin/agent-packs install local/frontend-engineer --dry-run
cli/bin/agent-packs list --target ./sandbox
cli/bin/agent-packs uninstall frontend-engineer --target ./sandbox
cli/bin/agent-packs doctor
cli/bin/agent-packs validate registry/packs
```

## Installation Model

Agent Packs orchestrates native install flows instead of replacing them.

- Local skills are copied into the selected agent skill target.
- Remote skills are fetched with `git` when the source is a Git URL or a GitHub `/tree/<branch>/<path>` URL.
- Plugin commands are always preview-safe by default and only run with `--execute-plugins`.
- Installed packs write receipts under `<target>/receipts/`.
- Installed packs write lockfiles under `<target>/packs/<pack-id>/agent-pack.lock`.
- `uninstall` removes installed skill folders and receipts; plugins are reported for native/manual cleanup.

## Remote Registries

Registries are named sources stored in `<target>/registries.json`.

```sh
cli/bin/agent-packs registry add official https://github.com/sandeshh/agent-packs --target ~/.agent-packs
cli/bin/agent-packs registry list --target ~/.agent-packs
cli/bin/agent-packs install official/frontend-engineer --target ~/.agent-packs
cli/bin/agent-packs registry remove official --target ~/.agent-packs
```

A registry source can be a local repository path or a Git URL. Remote registries are cloned into `<target>/registries/<name>/` and resolved from either `registry/packs/` or `packs/`.

## Specifying Plugins And Skills

Plugins and skills are declared as entries in `capabilities`. Plugin entries must include `format` and `install` metadata so an installer can resolve the marketplace/package/command. Skill entries must include `format` and `entry` so an installer can locate the `SKILL.md` file.

```json
{
  "type": "plugin",
  "name": "Anthropic Claude Code code-review plugin",
  "source": "https://github.com/anthropics/claude-plugins-official/tree/main/plugins/code-review",
  "format": "anthropic-plugin",
  "entry": ".claude-plugin/plugin.json",
  "requiresExecution": true,
  "trust": "official",
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

## Pack Composition

Packs can include other packs with the `packs` field. Included packs are expanded before install.

```json
{
  "id": "review-combo",
  "name": "Review Combo Pack",
  "version": "0.1.0",
  "description": "Composes review-oriented packs.",
  "packs": ["pr-review"]
}
```

## Examples

Example manifests live in `registry/schemas/examples/`:

- `minimal-pack.json`: the smallest valid pack manifest.
- `full-pack.json`: a complete manifest showing every supported capability type.
- `real-world-pack.json`: examples based on public Claude Code plugin and Agent Skills repositories.
- `composed-pack.json`: a pack that includes another pack.

## Tests

```sh
cd cli && go test ./...
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
