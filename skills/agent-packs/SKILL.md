---
name: agent-packs
description: Use when helping users install, configure, search, validate, author, publish, debug, or operate Agent Packs and its CLI, registry, packs, skills, plugins, policies, lockfiles, and supported coding-agent targets.
metadata:
  short-description: Help users operate Agent Packs
---

# Agent Packs

Use this skill when the user is working with Agent Packs itself: installing the CLI, choosing packs, installing capabilities into coding agents, creating registry entries, debugging validation failures, or preparing a registry contribution.

## First Checks

1. Locate the repo if the user is developing Agent Packs. Common path: `/Users/sandesh/dev/agent-packs`.
2. Prefer the built CLI at `cli/bin/agent-packs` inside the repo. Rebuild with:

```sh
cd cli
go build -o bin/agent-packs ./cmd/agent-packs
```

3. Before changing behavior, inspect `README.md`, `docs/architecture.md`, `registry/schemas/agent-pack.schema.json`, and the relevant package under `cli/internal/`.
4. After changes, run the smallest meaningful verification. Typical checks:

```sh
cd cli && go test ./...
python3 -m unittest discover -s tests
cli/bin/agent-packs validate registry/packs
cli/bin/agent-packs publish --check
```

## Core CLI Workflows

- Discover packs: `agent-packs search [query]`
- Explain a pack: `agent-packs show <pack> [--json]`
- Install a pack: `agent-packs install <pack> --agent codex --mode reference`
- Preview an install: `agent-packs install <pack> --dry-run`
- Initialize project defaults: `agent-packs init --agent codex --mode reference --scope project .`
- Validate manifests: `agent-packs validate registry/packs registry/skills registry/plugins`
- Inspect provenance: `agent-packs attribution <pack>` and `agent-packs licenses <pack>`
- Check safety: `agent-packs audit <pack>`, `agent-packs verify <pack>`, and `agent-packs policy check <pack> registry/policy/default.json`
- Compare installed state: `agent-packs diff <pack>` and `agent-packs outdated`
- Maintain installs: `agent-packs upgrade <pack>`, `agent-packs rollback <pack>`, `agent-packs uninstall <pack>`

## Registry Model

Agent Packs is registry-first:

- `registry/packs/`: pack manifests that compose capabilities.
- `registry/skills/<id>/SKILL.md`: reusable Agent Skill references.
- `registry/plugins/<id>/.claude-plugin/plugin.json`: reusable Claude Code plugin references.
- `registry/schemas/`: JSON Schema and examples.
- `registry/index.json`: generated searchable catalog.

Packs can include:

- `packs`: other pack IDs.
- `skills`: registry skill IDs or remote skill references.
- `plugins`: registry plugin IDs or remote plugin references.
- `capabilities`: inline skills, plugins, MCP servers, prompts, commands, hooks, templates, or tools.

Use `source` as the installable or resolvable location. Use `upstreamSource` only when separate attribution is helpful.

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
5. Regenerate `registry/index.json` with `agent-packs index --output registry/index.json` when catalog content changes.
6. Run `agent-packs publish --check` before pushing registry changes.

## Supported Targets

Use `agent-packs doctor targets` to inspect target directories. Common targets:

- Codex global skills: `.codex/skills`
- Codex project skills: `.agents/skills`
- Claude skills: `.claude/skills`
- Cursor skills: `.cursor/skills`
- GitHub Copilot skills: `.github/skills`

Use `--agent <tool>` or `--target-tool <tool>` to select a target. Use `--scope project` for project-local installs.

## Common Debugging

- If a pack is not found, check configured registries with `agent-packs registry list` and refresh with `agent-packs update --all`.
- If validation fails, compare against `registry/schemas/agent-pack.schema.json` and examples under `registry/schemas/examples/`.
- If audit warns about moving refs, decide whether the pack should pin a commit or intentionally track upstream.
- If Pages deploy fails in CI, ensure GitHub Pages is enabled and the repo variable `AGENT_PACKS_DEPLOY_PAGES` is set to `true`.
- If a local install changed unexpectedly, inspect the receipt and lockfile before reinstalling.
