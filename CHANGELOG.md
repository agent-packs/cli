# Changelog

All notable changes to the Agent Packs CLI are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This changelog covers the CLI (`agent-packs/cli`). Pack, skill, and plugin data
lives in the separate [`agent-packs/registry`](https://github.com/agent-packs/registry)
repository and is versioned independently.

## [Unreleased]

### Fixed
- **`pin --check` and `check` now fail closed.** A recorded pin whose source
  can no longer be resolved (deleted repo, unreachable network) is reported as
  `UNVERIFIABLE` and fails the gate; previously it silently passed as OK.
- **`check` fails on an empty target.** Running the CI gate against a target
  with no installed packs (e.g. from the wrong directory) now exits nonzero
  instead of passing vacuously.
- **`--min-trust` uses the registry trust taxonomy.** Enforcement previously
  ranked a non-existent `core/community/tap/unverified` scale, which scored
  `official` packs *below* `community`. It now ranks
  `official > verified > community > unknown` and rejects unknown levels.
- **`upgrade` no longer drops capability types.** The `--only` filter is
  recorded in the plan/receipt and replayed on upgrade; previously upgrade
  guessed from skill/plugin counts and silently removed memory, settings,
  command, and hook capabilities from mixed packs.
- **Integrity is verified before files land.** Skill checksums/signatures are
  checked against the materialized source before copy/symlink, so a mismatch
  never leaves tampered content in the agent's live skill directory.
- **An unreachable skill source now fails the install** (exit nonzero) instead
  of recording a `pending` capability and exiting 0.
- Opt-in analytics events are sent synchronously with a short timeout; the
  previous fire-and-forget goroutine raced process exit and rarely delivered.

### Added
- **Whole-tree skill checksums.** `agent-packs pin` records `dirsha256:`
  digests covering every file in a skill directory (legacy `sha256:` entry-file
  pins are still verified in their original format).
- **Registry provenance.** Receipts and lockfiles record the registry checkout
  commit (`registryCommit`) each install resolved packs from, and the default
  registry fetch prefers the latest release tag over the moving default branch
  when tags exist.
- `cap <kind> <install|list|upgrade|uninstall>` umbrella command for the
  standalone capability families (the per-kind commands remain as aliases),
  plus a reorganized `help` centered on the core workflow and `help --all` for
  the full surface.
- Reference-mode installs now print an explicit "recorded only — nothing was
  materialized" note so the default mode is never mistaken for a file install.
- CI runs the Go suite on Linux/macOS/Windows with gofmt + `go vet` gates and a
  built-binary smoke test.

### Changed
- Source materialization clones once per (repo, ref) and memoizes per run, and
  pin verification runs with bounded parallelism — `check`/`pin`/`outdated`
  no longer clone per capability sequentially.
- `git ls-remote` revision lookups are memoized per run and time-bounded.
- Flag parsing accepts flags and positionals in any order for every command,
  driven by each command's registered flag set (replaces the hand-maintained
  per-command argument reordering lists).
- `sync`, `freeze`, `export`, `why`, `test-run`, and `analytics` are marked
  experimental and print a notice; they may change or be removed.

### Removed
- The `hmac-sha256:` `integrity.signature` format. An HMAC is a shared-secret
  MAC, not a signature — anyone able to verify could forge — so it granted
  false assurance. `sha256:` checksums are unaffected; manifests declaring an
  hmac signature now fail verification with a clear error.

## [0.9.0] - 2026-06-24

### Added
- `prompt` and `template` capability types, completing the materialization of
  the schema's capability enum. Both are managed files (drift-tracked, cleanly
  uninstalled) installed to the portable `.agent-packs/prompts/*.md` and
  `.agent-packs/templates/*.md` directories; support `agent-packs new
  prompt|template` and `install --only prompts|templates`. No execution gate.

## [0.8.0] - 2026-06-23

### Added
- `subagent` capability type for distributing Claude Code subagents (delegated
  assistants defined by a markdown file with frontmatter). Installs to
  `.claude/agents/*.md` for Claude Code and the portable `.agent-packs/agents/*.md`
  for other agents; supports `agent-packs new subagent` and
  `install --only subagents`. Like commands, subagents are managed files with
  drift detection and clean uninstall; unlike hooks they run nothing, so no
  execution gate is required.

## [0.7.0] - 2026-06-23

### Added
- `agent-packs new` scaffolds the file-backed capability types added in v0.6.0:
  `command`, `hook`, `memory`, and `settings` (standalone capability JSON).
- `install --allow-hooks` gates writing hook capabilities in `--mode copy`.
  Installing a hook writes a file the target agent may run automatically, so it
  is opt-in (parallel to `--execute-plugins`); without the flag hooks are
  recorded with a content preview and a note, but not written.
- Install/dry-run plan output shows a content `preview` line for command and
  hook capabilities, plus a `note` for recorded items.
- `agent-packs init` now detects the agent in use (project-local signals) and
  the project stack (`go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`,
  …) and recommends matching packs by tag/category overlap. An explicit
  `--agent` wins; `--no-detect` writes flag defaults only.

## [0.6.0] - 2026-06-23

### Added
- File-backed `command` and `hook` capability types. In `reference` mode they
  are recorded only; with `--mode copy` Agent Packs writes the file from inline
  `content` or a materialized `source`, records a content hash, reports drift
  when the file is edited or removed, and deletes the managed file on
  uninstall/rollback.
- `install --only commands|hooks` filters plus shell completions for them.
- Target matrix `commandDestinations`/`hookDestinations`: Claude Code commands
  install to `.claude/commands/*.md`; other agents fall back to portable
  `.agent-packs/commands/*.md` and `.agent-packs/hooks/*.json` destinations
  unless a pack supplies an `agentTargets` override.

### Changed
- Release workflow links CLI releases to the Homebrew tap sync and requires a
  Homebrew tap token for the release dispatch.

## [0.5.0] - 2026-06-23

### Added
- Full documented file-backed memory/settings support for Claude Code, Codex,
  Gemini CLI, OpenCode, and GitHub Copilot.
- Rich target metadata for instruction and settings destinations, including
  scope, format, verification status, source documentation URL, and default
  destination markers.
- Codex TOML settings merge/retract/drift support with add-only, user-wins
  semantics.
- Copilot path-specific instruction support through `applyTo`, rendered as
  `.github/instructions/*.instructions.md` files with frontmatter.
- `install --only memory|settings` lifecycle support and JSON target matrix
  output via `agent-packs doctor targets --json`.

### Changed
- Memory/settings documentation now explains durable instruction files,
  reference-mode safety, generated-memory boundaries, and supported-agent
  caveats.
- Pack schema examples now include inline memory, Copilot instructions, Codex
  TOML settings, and JSON settings fragments.

### Fixed
- Avoid duplicate TOML table declarations when merging settings into an existing
  Codex config such as `[features]`.

## [0.4.0] - 2026-06-23

### Added
- **`memory` and `settings` capability types** (v1). Packs can now install agent
  memory (a managed markdown block appended to files like `CLAUDE.md`/`AGENTS.md`)
  and settings (a deep-merge into JSON config such as `.claude/settings.json`)
  across supported agents. Merges are idempotent, never clobber a user's existing
  keys/content (user-wins, add-only), and uninstall retracts only the
  pack-injected fragment so the file returns to its original state. Writes are
  atomic (temp-file + rename) and serialized with a per-file lock. `status`/drift
  reports edits to a pack-managed block or settings key. Unsupported
  (agent, type, scope) combinations skip with a recorded `unsupported` status.
  In the default `reference` mode merge capabilities are only recorded; applying
  them to a user's file requires an explicit `--mode copy`. Hooks and
  comment-preserving TOML/YAML settings are intentionally deferred to a later
  milestone.
- `agent-packs index --check` verifies that `index.json` is up to date without
  rewriting it, exiting non-zero on drift (useful in CI).
- Pack `categories` are now enforced against a canonical allowlist during
  `validate`, `lint`, and `publish --check`. The allowed set is read from the
  registry JSON schema when available, with a documented in-CLI fallback list.
- Trust-tier enforcement for object skill/plugin refs: every object ref must
  declare a `trust` value (`official`, `community`, or `verified`), validated by
  `validate`, `lint`, and `publish --check`. The enum is read from the registry
  schema with a documented fallback. Bare-string refs remain exempt.

### Changed
- Expanded automated test coverage across core CLI packages.

## [0.3.0] - 2026-06-15

### Added
- `list` commands now discover externally-installed skills and plugins (those
  installed outside Agent Packs), giving a complete view of an editor's
  capabilities.

### Changed
- Bumped landing-page version badge to v0.3.0.
- Updated GitHub Actions and Python test dependencies (Dependabot).

## [0.2.1] - 2026-06-14

### Fixed
- Flattened the Go module to the repository root so `go install` works against
  the CLI.

### Changed
- Updated Homebrew formula to v0.2.0.

## [0.2.0] - 2026-06-14

### Changed
- Split the registry data into the standalone `agent-packs/registry` repository
  and moved the Go module to `agent-packs/cli`. The CLI now fetches the registry
  at runtime instead of bundling it.
- Bumped landing-page badge to v0.2.0 and updated the Homebrew formula to v0.1.3.

## [0.1.3] - 2026-06-14

### Added
- GitHub governance configuration.

### Fixed
- Corrected the Docker registry path and added an actionable "registry not
  found" error message.
- Made the README `diff` example runnable against the sandbox install.

### Changed
- Hardened GitHub workflow permissions.
- Updated Homebrew formula to v0.1.2 and bumped the landing-page badge to v0.1.3.

## [0.1.2] - 2026-06-14

### Added
- Integrity pinning for capability sources and a deterministic registry index.
- Regression tests covering documented workflows and site commands.

### Fixed
- Corrected `status`/`scan` reporting bugs.
- Fixed the outdated history scan and broken site commands.
- Improved documentation and landing-page accuracy.

## [0.1.1] - 2026-06-14

### Fixed
- Registry-not-found error when the CLI was installed via Homebrew.
- Corrected Homebrew formula checksums for v0.1.0.

### Added
- Packs architecture section with an SVG diagram on the landing page.

## [0.1.0] - 2026-06-14

Initial public release of the Agent Packs CLI — a "Homebrew for agent
capabilities" that installs curated bundles of skills, plugins, prompts, and
templates into AI coding tools (Claude Code, Codex, Cursor, Copilot, Gemini CLI,
Goose, OpenCode).

### Added
- Core install pipeline: registry resolution, install planning, and execution
  with `reference`, `copy`, `symlink`, and `native` materialization modes.
- Receipt and lockfile tracking per install, enabling `upgrade`, `rollback`,
  `diff`, and `uninstall` flows.
- Pack composition via the `packs` field with recursive deduplication.
- Multi-pack and standalone skill/plugin lifecycle commands, including plugin
  uninstall support.
- Authoring and publishing workflow: `new`, `validate`, `lint`, `verify`,
  `audit`, `publish --check`, and policy presets.
- Drift detection, shell completions, deprecation warnings, `upgrade --all`,
  `why`, `doctor` (with `--json`), `sync`/`freeze`, custom targets, version
  pinning, search filtering, and export / install-from.
- Bundled `agent-packs` helper skill installed into supported editors during
  bootstrap.
- Docker image and Homebrew tap publishing; GitHub Pages landing page with a
  searchable catalog.
- `CLAUDE.md` with build, test, and architecture guidance.

[Unreleased]: https://github.com/agent-packs/cli/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/agent-packs/cli/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/agent-packs/cli/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/agent-packs/cli/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/agent-packs/cli/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/agent-packs/cli/compare/v0.1.3...v0.2.0
[0.1.3]: https://github.com/agent-packs/cli/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/agent-packs/cli/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/agent-packs/cli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/agent-packs/cli/releases/tag/v0.1.0
