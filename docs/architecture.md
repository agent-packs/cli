# Architecture

Agent Packs should feel like Homebrew for agent capabilities while keeping the CLI and registry separable.

## Production Stack

- CLI: Go module under `cli/` split into focused packages (`model`, `registry`, `resolve`, `plan`, `install`, `policy`, `validate`, `targets`, `config`, `output`, `version`).
- Registry packs: static JSON manifests under `registry/packs/`.
- Registry skills: Agent Skill source references under `registry/skills/<id>/SKILL.md`.
- Registry plugins: Claude Code plugin source references under `registry/plugins/<id>/.claude-plugin/plugin.json`.
- Schema: `registry/schemas/agent-pack.schema.json`.
- Catalog metadata: maintainers, stability, review status, deprecation, replacement, last verified date, and tool/version requirements.
- Policy defaults: `registry/policy/default.json`.
- Receipts: `<target>/receipts/<pack-id>.json`.
- Lockfiles: `<target>/packs/<pack-id>/agent-pack.lock`, including source revision fields when locally resolvable.
- Project config: `.agent-packs.yaml` for default agent, mode, scope, and target.

## CLI Commands

Implemented commands:

- `agent-packs search [query] [--json]`
- `agent-packs show <pack> [--json]`
- `agent-packs install <pack|registry/pack>`
- `agent-packs skills install <skill-id|path>`
- `agent-packs plugins install <plugin-id|path>`
- `agent-packs list [--json]`
- `agent-packs uninstall <pack>`
- `agent-packs upgrade <pack>`
- `agent-packs rollback <pack>`
- `agent-packs audit <pack> [--json]`
- `agent-packs version [--json]`
- `agent-packs init [dir]`
- `agent-packs new <pack|skill|plugin> <id>`
- `agent-packs doctor`
- `agent-packs doctor targets`
- `agent-packs validate <file-or-directory>`
- `agent-packs registry add <name> <source>`
- `agent-packs registry list`
- `agent-packs registry remove <name>`
- `agent-packs update --all`
- `agent-packs outdated [--json]`
- `agent-packs cache`
- `agent-packs scan [path]`
- `agent-packs import <skills-dir>`
- `agent-packs lint <pack>`
- `agent-packs verify <pack>`
- `agent-packs resolve <pack>`
- `agent-packs tree|deps <pack> [--json]`
- `agent-packs publish --check [--json]`
- `agent-packs policy check <pack> <policy.json|preset>`
- `agent-packs licenses <pack>`
- `agent-packs attribution <pack>`
- `agent-packs index [--output path]`
- `agent-packs diff <pack>`
- `agent-packs pin <pack> [--check]`
- `agent-packs compat <pack> [--json]`
- `agent-packs cache prune|clean`

## Install Experience

Target install experience:

```sh
brew install sandeshh/agent-packs/agent-packs
agent-packs install frontend-engineer
```

Bootstrap fallback:

```sh
curl -fsSL https://raw.githubusercontent.com/sandeshh/agent-packs/main/install.sh | sh
```

Release binaries are built by `.github/workflows/release.yml` on version tags (`v*`).
Release archives bundle both the `skills/agent-packs` skill and the `registry/`
so the binary works standalone. The bootstrap installer installs the skill to
the selected editor's skill directory with `AGENT_PACKS_AGENT` (defaults to
Codex) and copies the registry to `~/.local/share/agent-packs/registry`. The
Homebrew formula installs the registry under `share/agent-packs/registry`; the
CLI resolves the default registry from `<prefix>/share/agent-packs/registry`
when `AGENT_PACKS_REGISTRY` is unset.

## Catalog And CI

CI (`.github/workflows/ci.yml`) runs Go and Python tests, JSON Schema validation, registry validation, pack verification, audit, policy checks, index generation, index staleness checks, and GitHub Pages deployment for the static catalog.

`docs/catalog.html` renders `registry/index.json` as a lightweight catalog.

## Security Posture

Plugin install and uninstall commands are not executed unless the user passes `--execute-plugins`. Plugin execution uses a timeout, respects `AGENT_PACKS_PLUGIN_CWD`, and supports structured handlers for `claude-marketplace` and `manual` lifecycle methods.

Standalone `skills` and `plugins` commands write receipts under `<target>/receipts/skills/` and `<target>/receipts/plugins/`, keeping independent capability lifecycle state separate from pack receipts.

Plugin capabilities with install or uninstall commands should set `requiresExecution: true` and should include trust metadata such as `trust: "official"` or `trust: "community"`.

Integrity metadata uses `integrity.checksum` (`sha256:`) and optional `integrity.signature` (`sha256:` or `hmac-sha256:` with `AGENT_PACKS_TRUST_KEY`). Checksums are verified after skill materialization.

The target matrix maps supported tools to global and project skill directories, with aliases such as `claude-code` → `claude`. Registry skills and plugins are referenced from their upstream source and are not copied into the selected agent target by default (`--mode reference`).

Remote sources support GitHub tree/commit URLs, GitLab tree URLs, generic git URLs, and archive downloads (`.tar.gz`, `.zip`). Moving refs can be resolved live with `git ls-remote` for outdated reporting.

## Why Go

Go is the safest default for a brew-like developer tool:

- Single binary distribution.
- Fast startup.
- Good filesystem and archive support.
- Easy release automation with GitHub Actions.
- Clean cross-compilation.
- Familiar enough for infrastructure-oriented contributors.
