# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```sh
# Build the CLI
go build -o bin/agent-packs ./cmd/agent-packs

# Run Go unit tests
go test ./...

# Run a single Go test file or package
go test ./internal/install/...

# Run Python integration and docs tests (requires venv)
python3 -m venv .venv && .venv/bin/pip install -r tests/requirements.txt
.venv/bin/python -m unittest discover -s tests

# Run a single Python test file
.venv/bin/python -m unittest tests.test_install

# Run the CLI against a local registry checkout (registry data is a separate repo)
git clone https://github.com/agent-packs/registry /tmp/registry
AGENT_PACKS_REGISTRY=/tmp/registry/packs bin/agent-packs validate packs
```

## Repository Split

This repo (`agent-packs/cli`) holds **only the CLI**. The pack/skill/plugin data
lives in a separate repo, **`agent-packs/registry`**. The CLI fetches the registry
at runtime (`internal/registry/fetch.go`) into the user cache on first use;
`AGENT_PACKS_REGISTRY` overrides with a local `packs/` path,
`AGENT_PACKS_REGISTRY_REPO`/`_REF` override the source repo/ref. There is no
`registry/` directory in this repo — validate/index/authoring commands run against
a checkout of `agent-packs/registry`.

## Architecture

Agent Packs is a CLI tool (think "Homebrew for agent capabilities") that installs curated bundles of agent skills, plugins, prompts, and templates into AI coding tools (Claude Code, Codex, Cursor, Copilot, Gemini CLI, Goose, OpenCode).

### CLI (repo root)

Go module with a single binary entry point at `cmd/agent-packs/main.go`. Internal packages follow a strict layered dependency: `model` → `registry`/`resolve`/`targets` → `plan` → `install` → commands.

- **`model/`** — core data types: `Pack`, `Capability`, `CapabilityRef`, `SkillManifest`, `PluginManifest`, install options, receipts, lockfiles, and report types.
- **`registry/`** — loads and searches JSON manifests from `registry/packs/`; resolves named and `registryname/pack-id` refs; expands composed packs (deduplicating sub-packs); manages remote registries stored in `<target>/registries.json`.
- **`resolve/`** — classifies capability sources as local, GitHub tree, pinned commit, or moving ref; supports `git ls-remote` for staleness checks.
- **`targets/`** — maps tool IDs (`claude`, `codex`, `cursor`, etc.) to global and project skill directories; handles aliases like `claude-code` → `claude`.
- **`plan/`** — builds an `InstallPlan` from an expanded pack; maps capabilities to target paths based on agent, mode (`reference`/`symlink`/`copy`/`native`), and `--only` filter.
- **`install/`** — executes plans: materializes skills (copy/symlink/reference), runs plugin install commands (gated by `--execute-plugins`), writes receipts under `<target>/receipts/` and lockfiles under `<target>/packs/<id>/agent-pack.lock`.
- **`agentpacks/`** — higher-level command implementations wiring the above together (search, show, audit, verify, lint, diff, outdated, publish check, etc.).
- **`validate/`**, **`policy/`**, **`author/`**, **`config/`**, **`output/`**, **`version/`** — validation against JSON schema, policy enforcement, scaffolding new manifests, project config (`.agent-packs.yaml`), output formatting, and version info.

### Registry (separate repo: `agent-packs/registry`)

The registry **data is not in this repo** — it lives in
[`agent-packs/registry`](https://github.com/agent-packs/registry) (`packs/`,
`skills/`, `plugins/`, `schemas/`, `policy/`, `index.json`). The CLI fetches it at
runtime via `internal/registry/fetch.go` (`EnsureLocalRegistry`) into the user
cache; `agent-packs update` refreshes it. Schema/manifest validation tests
(`test_schema.py`, `test_jsonschema.py`) live in that repo. `index.json` must be
regenerated there whenever packs change.

### Skills (`skills/`)

The bundled `agent-packs` skill (`skills/agent-packs/SKILL.md`) is installed into supported editors' skill directories during bootstrap so agents can help users with the CLI itself.

### Tests (`tests/`)

Python tests using `unittest`:
- `test_install.py` — integration tests that build the CLI binary and exercise install/upgrade/rollback/uninstall flows with temp registries (each sets `AGENT_PACKS_REGISTRY`, so no network).
- `test_bundled_skill.py` — validates the bundled `agent-packs` skill.
- `test_docs.py` — guards README/landing-page accuracy.

## Key Conventions

**The registry is fetched at runtime, not bundled.** Releases ship only the binary + bundled skill; the CLI clones `agent-packs/registry` on first use. Override with `AGENT_PACKS_REGISTRY` (local `packs/` path), `AGENT_PACKS_REGISTRY_REPO`, or `AGENT_PACKS_REGISTRY_REF`.

**Skills and plugins in packs are source references, not copies.** Registry `skills/` and `plugins/` entries are only referenced; inline `capabilities` in pack manifests are what gets materialized. Use `--mode reference` (default) to record sources without copying, `--mode copy` to materialize, `--mode symlink` to symlink, `--mode native` for plugin install commands.

**Plugin execution is opt-in.** Plugin install commands only run with `--execute-plugins`; without it, commands are recorded in the plan but not executed.

**Pack composition via `packs` field.** A pack can include other packs by ID; `registry.ExpandPack` recursively deduplicates before planning. The `skills` and `plugins` fields in a pack manifest are shorthand references resolved from the local registry or from object refs with remote `source` URLs.

**Receipt + lockfile pattern.** Every install writes `<target>/receipts/<pack-id>.json` (human-readable install record) and `<target>/packs/<pack-id>/agent-pack.lock` (machine-readable state for upgrade/rollback/diff).
