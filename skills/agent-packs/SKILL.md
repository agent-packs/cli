---
name: agent-packs
description: Use when helping users install, configure, search, validate, author, publish, debug, or operate Agent Packs and its CLI, registry, packs, skills, plugins, policies, lockfiles, and supported coding-agent targets.
metadata:
  short-description: Help users operate Agent Packs
---

# Agent Packs

Use this skill when the user is working with Agent Packs itself: installing the CLI, choosing packs, installing capabilities into agentic code editors, creating registry entries, debugging validation failures, or preparing a registry contribution.

## First Checks

1. Locate the repo if the user is developing Agent Packs. Common path: `/Users/sandesh/dev/agent-packs`.
2. Prefer the built CLI at `bin/agent-packs` inside the repo. Rebuild with:

```sh
go build -o bin/agent-packs ./cmd/agent-packs
```

3. Before changing behavior, inspect `README.md`, `docs/architecture.md`, and the relevant package under `internal/`. The pack/skill/plugin data and JSON Schema live in the separate `agent-packs/registry` repo.
4. After changes, run the smallest meaningful verification. Typical checks:

```sh
go test ./...
python3 -m unittest discover -s tests
# Registry authoring/validation runs against a checkout of agent-packs/registry:
AGENT_PACKS_REGISTRY=/path/to/registry/packs agent-packs validate packs
```

## Core CLI Workflows

- Discover packs: `agent-packs search [query]`
- Explain a pack: `agent-packs show <pack> [--json]`
- Install a pack: `agent-packs install <pack> --agent <tool> --mode reference`
- Preview an install: `agent-packs install <pack> --dry-run`
- Apply memory/settings merges (default reference mode only records them): `agent-packs install <pack> --agent <tool> --mode copy`
- Initialize project defaults: `agent-packs init --agent <tool> --mode reference --scope project .`
- Validate manifests (in a registry checkout): `agent-packs validate packs skills plugins`
- Inspect provenance: `agent-packs attribution <pack>` and `agent-packs licenses <pack>`
- Check safety: `agent-packs audit <pack>`, `agent-packs verify <pack>`, and `agent-packs policy check <pack> default`
- Compare installed state: `agent-packs diff <pack>` and `agent-packs outdated`
- Maintain installs: `agent-packs upgrade <pack>`, `agent-packs rollback <pack>`, `agent-packs uninstall <pack>`

## Registry Model

Agent Packs is registry-first. The registry data lives in the separate
[`agent-packs/registry`](https://github.com/agent-packs/registry) repo, which the
CLI fetches at runtime (override with `AGENT_PACKS_REGISTRY`,
`AGENT_PACKS_REGISTRY_REPO`, or `AGENT_PACKS_REGISTRY_REF`). In that repo:

- `packs/`: pack manifests that compose capabilities.
- `skills/<id>/SKILL.md`: reusable Agent Skill references.
- `plugins/<id>/.claude-plugin/plugin.json`: reusable Claude Code plugin references.
- `schemas/`: JSON Schema and examples.
- `index.json`: generated searchable catalog.

Packs can include:

- `packs`: other pack IDs.
- `skills`: registry skill IDs or remote skill references.
- `plugins`: registry plugin IDs or remote plugin references.
- `capabilities`: inline skills, plugins, prompts, commands, hooks, templates, or tools.

Use `source` as the installable or resolvable location. Use `upstreamSource` only when separate attribution is helpful.

`memory` and `settings` are merge capabilities: instead of installing a file, they merge a fragment into a file the agent already owns. A `memory` capability writes an idempotent managed markdown block into the agent's memory file (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, …); a `settings` capability deep-merges a JSON fragment into the agent's settings (e.g. `.claude/settings.json`), optionally scoped by `mergeKey`. Provide the fragment inline via `content`, or via a `source` file. Merges are user-wins/add-only and only happen with `--mode copy` (the default `reference` mode records intent without touching the file); uninstall retracts only what the pack injected. Run `agent-packs targets` to see which agents support each type. Hooks and TOML/YAML settings are not yet supported.

## Install Model

Default to safe plans:

- `reference`: record sources without copying.
- `symlink`: link materialized skills.
- `copy`: copy materialized skills.
- `native`: plan native plugin installs.

Plugin commands are preview-safe by default. Do not execute plugin commands unless the user explicitly asks and passes `--execute-plugins`.

Installed packs write:

- receipts under `<target>/receipts/`
- lockfiles under `<target>/packs/<pack-id>/agent-pack.lock`

## Authoring Guidance

When adding or changing a pack:

1. Prefer remote source references over copying upstream skill or plugin content.
2. Pin source refs when reproducibility matters. Moving refs are acceptable only when the pack intentionally tracks upstream.
3. Add `trust`, `license`, `homepage` or `repository`, and `upstreamSource` where useful.
4. Keep pack metadata searchable with `tags`, `categories`, `tools`, `maintainers`, `stability`, `reviewStatus`, and `lastVerified`.
5. Regenerate the registry `index.json` (in an `agent-packs/registry` checkout) with `agent-packs index --output index.json` when catalog content changes.
6. Run `agent-packs publish --check` before pushing registry changes.

## Supported Agentic Code Editors

Use `agent-packs doctor targets` to inspect target directories. Common targets:

- Codex global skills: `.codex/skills`
- Codex project skills: `.agents/skills`
- Claude skills: `.claude/skills`
- Cursor skills: `.cursor/skills`
- Gemini CLI skills: `.gemini/skills`
- GitHub Copilot skills: `.github/skills`
- Goose skills: `.goose/skills`
- OpenCode skills: `.opencode/skills`
- Generic skills: `skills`

Use `--agent <tool>` or `--target-tool <tool>` to select a target. Supported tool IDs include `codex`, `claude`, `cursor`, `gemini`, `copilot`, `goose`, `opencode`, and `generic`. Common CLI aliases include `claude-code` and `github-copilot`. Use `--scope project` for project-local installs.

When helping users install this bundled `agent-packs` skill, prefer the bootstrap environment variable over hardcoded paths:

```sh
curl -fsSL https://raw.githubusercontent.com/agent-packs/cli/main/install.sh | AGENT_PACKS_AGENT=opencode sh
```

Use `AGENT_PACKS_SKILL_DIR=/path/to/skills/agent-packs` only when the editor uses a custom skill location.

## Common Debugging

- If a pack is not found, check configured registries with `agent-packs registry list` and refresh with `agent-packs update --all`.
- If validation fails, compare against the registry repo's `schemas/agent-pack.schema.json` and examples under `schemas/examples/`.
- If audit warns about moving refs, decide whether the pack should pin a commit or intentionally track upstream.
- If Pages deploy fails in CI, ensure GitHub Pages is enabled and the repo variable `AGENT_PACKS_DEPLOY_PAGES` is set to `true`.
- If a local install changed unexpectedly, inspect the receipt and lockfile before reinstalling.
