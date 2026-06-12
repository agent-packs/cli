# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```sh
# Build the CLI
cd cli && go build -o bin/agent-packs ./cmd/agent-packs

# Run Go unit tests
cd cli && go test ./...

# Run a single Go test file or package
cd cli && go test ./internal/install/...

# Run Python schema and integration tests (requires venv)
python3 -m venv .venv && .venv/bin/pip install -r tests/requirements.txt
.venv/bin/python -m unittest discover -s tests

# Run a single Python test file
.venv/bin/python -m unittest tests.test_install

# Validate registry manifests
cli/bin/agent-packs validate registry/packs
cli/bin/agent-packs validate registry/skills
cli/bin/agent-packs validate registry/plugins

# Regenerate registry index after adding/modifying packs (must be kept in sync)
cli/bin/agent-packs index --output registry/index.json
```

## Architecture

Agent Packs is a CLI tool (think "Homebrew for agent capabilities") that installs curated bundles of agent skills, plugins, prompts, and templates into AI coding tools (Claude Code, Codex, Cursor, Copilot, Gemini CLI, Goose, OpenCode).

### CLI (`cli/`)

Go module with a single binary entry point at `cli/cmd/agent-packs/main.go`. Internal packages follow a strict layered dependency: `model` → `registry`/`resolve`/`targets` → `plan` → `install` → commands.

- **`model/`** — core data types: `Pack`, `Capability`, `CapabilityRef`, `SkillManifest`, `PluginManifest`, install options, receipts, lockfiles, and report types.
- **`registry/`** — loads and searches JSON manifests from `registry/packs/`; resolves named and `registryname/pack-id` refs; expands composed packs (deduplicating sub-packs); manages remote registries stored in `<target>/registries.json`.
- **`resolve/`** — classifies capability sources as local, GitHub tree, pinned commit, or moving ref; supports `git ls-remote` for staleness checks.
- **`targets/`** — maps tool IDs (`claude`, `codex`, `cursor`, etc.) to global and project skill directories; handles aliases like `claude-code` → `claude`.
- **`plan/`** — builds an `InstallPlan` from an expanded pack; maps capabilities to target paths based on agent, mode (`reference`/`symlink`/`copy`/`native`), and `--only` filter.
- **`install/`** — executes plans: materializes skills (copy/symlink/reference), runs plugin install commands (gated by `--execute-plugins`), writes receipts under `<target>/receipts/` and lockfiles under `<target>/packs/<id>/agent-pack.lock`.
- **`agentpacks/`** — higher-level command implementations wiring the above together (search, show, audit, verify, lint, diff, outdated, publish check, etc.).
- **`validate/`**, **`policy/`**, **`author/`**, **`config/`**, **`output/`**, **`version/`** — validation against JSON schema, policy enforcement, scaffolding new manifests, project config (`.agent-packs.yaml`), output formatting, and version info.

### Registry (`registry/`)

Static data — not Go source:

- **`packs/`** — one JSON manifest per pack (e.g. `frontend-engineer.json`). Each declares `id`, `name`, `version`, `description`, optional `packs` (composed sub-packs), `skills`/`plugins` (source references by registry ID or object with remote `source`), and `capabilities` (inline installable items).
- **`skills/<id>/SKILL.md`** — reusable Agent Skills with required frontmatter (`name`, `description`). `metadata.agentpacks.source` points at the upstream remote source.
- **`plugins/<id>/.claude-plugin/plugin.json`** — reusable Claude Code plugins with `repository`/`homepage` for upstream provenance.
- **`schemas/agent-pack.schema.json`** — authoritative JSON Schema for pack validation.
- **`schemas/examples/`** — canonical example manifests (`minimal-pack.json`, `full-pack.json`, `real-world-pack.json`, `composed-pack.json`, `referenced-capabilities-pack.json`).
- **`index.json`** — pre-generated searchable index; must be regenerated and committed whenever packs change (CI checks for staleness).
- **`policy/default.json`** — default policy rules for `policy check`.

### Skills (`skills/`)

The bundled `agent-packs` skill (`skills/agent-packs/SKILL.md`) is installed into supported editors' skill directories during bootstrap so agents can help users with the CLI itself.

### Tests (`tests/`)

Python tests using `unittest`:
- `test_schema.py` — validates all registry manifests against the JSON Schema and structural rules.
- `test_install.py` — integration tests that build the CLI binary and exercise install/upgrade/rollback/uninstall flows with temp registries.
- `test_jsonschema.py` — validates the JSON Schema file itself.
- `test_bundled_skill.py` — validates the bundled `agent-packs` skill.

## Key Conventions

**`registry/index.json` must be regenerated** after any registry change: `cli/bin/agent-packs index --output registry/index.json`. CI fails if the committed index is stale.

**Skills and plugins in packs are source references, not copies.** `registry/skills/` and `registry/plugins/` entries are only referenced; inline `capabilities` in pack manifests are what gets materialized. Use `--mode reference` (default) to record sources without copying, `--mode copy` to materialize, `--mode symlink` to symlink, `--mode native` for plugin install commands.

**Plugin execution is opt-in.** Plugin install commands only run with `--execute-plugins`; without it, commands are recorded in the plan but not executed.

**Pack composition via `packs` field.** A pack can include other packs by ID; `registry.ExpandPack` recursively deduplicates before planning. The `skills` and `plugins` fields in a pack manifest are shorthand references resolved from the local registry or from object refs with remote `source` URLs.

**Receipt + lockfile pattern.** Every install writes `<target>/receipts/<pack-id>.json` (human-readable install record) and `<target>/packs/<pack-id>/agent-pack.lock` (machine-readable state for upgrade/rollback/diff).
