# Agent Packs

Curated, installable capability bundles for AI coding agents.

Agent Packs bundles public Skills, Plugins, commands, hooks, prompts, templates,
and composed packs into ready-to-use workflow packs.

![Agent Packs high-level architecture](docs/architecture.svg)

Agent Packs aggregates capabilities from public skill repositories, Claude/Codex plugins,
prompts, templates, commands, hooks, and remote registries. The CLI
normalizes those sources into registry-backed packs, preserves provenance, writes
receipts and lockfiles, and syncs capabilities into supported coding agents through
reference, symlink, copy, or native install modes.

## Repository Layout

This repository (`agent-packs/cli`) holds the CLI. The pack/skill/plugin data
lives in a separate repo,
[`agent-packs/registry`](https://github.com/agent-packs/registry), which the CLI
fetches at runtime.

- `cli/`: Go CLI module and source.
- `skills/agent-packs/`: the bundled `agent-packs` skill installed into editors.
- `docs/`: architecture notes and the GitHub Pages site.
- `tests/`: Python CLI integration, bundled-skill, and docs tests.

The registry data (`packs/`, `skills/`, `plugins/`, `schemas/`, `policy/`,
`index.json`) is maintained in
[`agent-packs/registry`](https://github.com/agent-packs/registry).

## Build

```sh
cd cli
go build -o bin/agent-packs ./cmd/agent-packs
```

## Install

Homebrew:

```sh
brew install agent-packs/tap/agent-packs
```

Bootstrap installer:

```sh
curl -fsSL https://raw.githubusercontent.com/agent-packs/cli/main/install.sh | sh
```

Or build locally from source (see Build above).

The release archive also includes a bundled `agent-packs` skill for agentic code
editors. The bootstrap installer copies it to `~/.codex/skills/agent-packs` by
default so agents can help users search, install, author, validate, and debug
Agent Packs. Set `AGENT_PACKS_AGENT` to install it for another supported editor,
`AGENT_PACKS_INSTALL_SKILL=0` to skip this step, or `AGENT_PACKS_SKILL_DIR` to
choose a custom skill directory.

```sh
curl -fsSL https://raw.githubusercontent.com/agent-packs/cli/main/install.sh | AGENT_PACKS_AGENT=opencode sh
curl -fsSL https://raw.githubusercontent.com/agent-packs/cli/main/install.sh | AGENT_PACKS_AGENT=claude sh
```

## Registry

The pack/skill/plugin registry lives in a separate repo,
[`agent-packs/registry`](https://github.com/agent-packs/registry). The CLI fetches
it on first use and caches it under your user cache dir (e.g.
`~/.cache/agent-packs/registry`); `agent-packs update` refreshes that cache.

Override registry resolution with environment variables:

- `AGENT_PACKS_REGISTRY` — path to a local `packs/` directory (skips fetching;
  ideal for working on the registry or for air-gapped/offline use).
- `AGENT_PACKS_REGISTRY_REPO` — a different registry repo URL (e.g. a fork).
- `AGENT_PACKS_REGISTRY_REF` — pin to a branch, tag, or commit for reproducibility.

To author packs, clone the registry and point the CLI at it:

```sh
git clone https://github.com/agent-packs/registry
cd registry
AGENT_PACKS_REGISTRY=./packs agent-packs validate packs
AGENT_PACKS_REGISTRY=./packs agent-packs new pack my-pack --dir packs
AGENT_PACKS_REGISTRY=./packs agent-packs index --output index.json
```

## CLI Usage

```sh
cli/bin/agent-packs search
cli/bin/agent-packs show frontend-engineer
cli/bin/agent-packs install frontend-engineer --target ./sandbox
cli/bin/agent-packs install frontend-engineer pr-review popular-engineering-skills --target ./sandbox
cli/bin/agent-packs install frontend-engineer --agent codex --only skills --dry-run
cli/bin/agent-packs skills install ./my-skill --agent codex --mode copy --target ./sandbox
cli/bin/agent-packs plugins install claude-code-review --mode native --method claude-marketplace --marketplace claude-plugins-official --package code-review --dry-run
cli/bin/agent-packs init --agent codex --mode reference --scope project .
cli/bin/agent-packs version
```

Additional commands:

```sh
cli/bin/agent-packs search frontend --json
cli/bin/agent-packs show frontend-engineer --json
cli/bin/agent-packs audit frontend-engineer --json
cli/bin/agent-packs upgrade frontend-engineer pr-review --target ./sandbox
cli/bin/agent-packs rollback frontend-engineer pr-review --target ./sandbox
cli/bin/agent-packs tree eng-leader
cli/bin/agent-packs publish --check
cli/bin/agent-packs registry add local /path/to/agent-packs
cli/bin/agent-packs install local/frontend-engineer --dry-run
cli/bin/agent-packs install eng-leader --target-tool codex --mode symlink --on-conflict backup --project .
cli/bin/agent-packs cache
cli/bin/agent-packs update --all
cli/bin/agent-packs outdated
cli/bin/agent-packs scan ~/.codex/skills
cli/bin/agent-packs import ~/.codex/skills
cli/bin/agent-packs lint eng-leader
cli/bin/agent-packs verify eng-leader
cli/bin/agent-packs resolve eng-leader
cli/bin/agent-packs policy check eng-leader default
cli/bin/agent-packs licenses eng-leader
cli/bin/agent-packs attribution eng-leader
cli/bin/agent-packs diff frontend-engineer --target ./sandbox
cli/bin/agent-packs pin frontend-engineer --target ./sandbox
cli/bin/agent-packs pin frontend-engineer --target ./sandbox --check
cli/bin/agent-packs compat eng-leader --agent codex
cli/bin/agent-packs cache prune
cli/bin/agent-packs list --target ./sandbox
cli/bin/agent-packs uninstall frontend-engineer pr-review --target ./sandbox
cli/bin/agent-packs doctor
cli/bin/agent-packs doctor targets
```

## Included Packs

- `frontend-engineer`: frontend implementation and browser verification workflows.
- `frontend-quality`: frontend UI engineering, browser verification, and performance checks.
- `pr-review`: code review and pull request inspection workflows.
- `eng-leader`: engineering leadership workflows for strategy, planning, quality, architecture decisions, delivery, launch readiness, security, and performance. Several skills reference Addy Osmani's public `addyosmani/agent-skills` repository via upstream source metadata.
- `product-discovery`: interview, idea refinement, spec writing, and task breakdown workflows.
- `implementation-core`: context engineering, source-grounded development, API design, incremental implementation, and TDD workflows.
- `reliability-debugging`: debugging, adversarial review, test hardening, and security hardening workflows.
- `shipping-ops`: git workflow, CI/CD, launch readiness, migrations, and ADR/documentation workflows.
- `full-lifecycle-engineer`: composed lifecycle pack from discovery through implementation, review, release, and follow-up.
- `popular-engineering-skills`: broadly useful public engineering skills selected from high-visibility lifecycle skill repositories.
- `popular-claude-dev-plugins`: common Claude Code development plugins from Anthropic's official plugin directory.
- `popular-integration-plugins`: common Claude Code integration plugins for GitHub, GitLab, Playwright, Context7, Linear, Firebase, Terraform, and semantic code tooling.
- `popular-agent-starter`: a composed starter bundle combining popular engineering skills and Claude Code development plugins.
- `ai-engineer`, `backend-engineer`, `cloud-infrastructure`, `data-engineer`, `database-reliability`, `devex-engineer`, `ml-engineer`, `mobile-engineer`, `open-source-maintainer`, `platform-engineer`, `qa-engineer`, `rust-engineer`, `security-engineer`, and `sre-engineer`: role-focused packs for common engineering specialties.
- `superpowers`: a composed pack for `obra/superpowers`, plus focused Superpowers workflow, quality/debugging, collaboration/subagent, meta-authoring, and distribution-plugin packs.

The `popular-*` packs use public proxies for adoption because most skill/plugin
ecosystems do not publish install counts. Selection signals include official or
curated marketplace presence, repository popularity, broad workflow coverage, and
common agentic coding use cases.

## Tool Target Matrix

Agent Packs knows common global and project skill directories for supported coding tools. Use `agent-packs doctor targets` to inspect the matrix. Install with `--agent <tool>` or `--target-tool <tool>`. Project installs use the tool's project skill directory, for example Codex project installs target `.agents/skills`.

Supported tools include `codex`, `claude`, `cursor`, `gemini`, `copilot`, `goose`, `opencode`, and `generic`. Common aliases such as `claude-code` map to `claude`.

## Project Configuration

Initialize a project config with defaults for agent, sync mode, and install scope:

```sh
cli/bin/agent-packs init --agent codex --mode reference --scope project .
```

This writes `.agent-packs.yaml` in the project directory.

## Installation Model

Agent Packs orchestrates native install flows instead of replacing them.

- Pack-level `skills` and `plugins` entries are source references. They are recorded in plans, receipts, and lockfiles, but are not copied into the target.
- `source` is the location or command the installer resolves. Prefer pinned commit refs for reproducibility.
- Optional `upstreamSource` is only for attribution/provenance when `source` is not enough.
- Registry skills point at remote sources with `metadata.agentpacks.source`.
- Registry plugins reference their `repository` or `homepage` when available; otherwise they reference their registry directory.
- Inline local skill capabilities can still be copied into the selected agent skill target when a pack explicitly declares them under `capabilities`.
- Inline remote skill capabilities can still be fetched with `git` when the source is a Git URL or a GitHub `/tree/<branch>/<path>` URL.
- Sync modes are explicit: `reference` records sources only, `symlink` links materialized skills, `copy` copies skills, and `native` enables native plugin planning.
- Conflicts are controlled with `--on-conflict skip|overwrite|backup`.
- Inline plugin install and uninstall commands are preview-safe by default and only run with `--execute-plugins`.
- Lifecycle commands accept multiple pack IDs directly: `install`, `upgrade`, `rollback`, and `uninstall` run packs sequentially and fail fast on the first error.
- Installed packs write receipts under `<target>/receipts/`.
- Installed packs write lockfiles under `<target>/packs/<pack-id>/agent-pack.lock`.
- `uninstall` removes installed inline skill folders and receipts. Plugin cleanup commands run only with `uninstall --execute-plugins`; otherwise plugins are reported for native/manual cleanup.
- Standalone `skills` and `plugins` commands manage independent capabilities under `<target>/receipts/skills/` and `<target>/receipts/plugins/` without requiring a pack wrapper.

## Lifecycle Commands

Agent Packs supports a basic package-manager lifecycle:

- `cache`: shows and creates central source/cache/lock/registry directories under the Agent Packs home.
- `update --all`: refreshes configured registries.
- `outdated`: lists installed packs with pack-version drift and capability revision drift (`--json` supported).
- `install <pack...>`: installs one or more packs with shared target, agent, mode, conflict, and plugin execution settings.
- `upgrade <pack...>`: re-installs one or more packs using each pack's prior receipt settings.
- `rollback <pack...>`: restores one or more previous receipt-backed install states when history exists.
- `uninstall <pack...>`: removes one or more installed packs, including installed inline skill folders, optional plugin cleanup commands, and receipts.
- `skills install|list|upgrade|uninstall`: manages independent registry or local Agent Skills without wrapping them in a pack.
- `plugins install|list|upgrade|uninstall`: manages independent registry or local plugins; pass `--method`, `--package`, `--marketplace`, `--command`, or `--uninstall-command` to store native lifecycle metadata outside a pack.
- `audit <pack>`: supply-chain SBOM report (`--json` supported).
- `version`: prints CLI version (`--json` supported).
- `init [dir]`: writes `.agent-packs.yaml` project defaults.
- `new pack|skill|plugin <id>`: scaffolds valid starter manifests.
- `tree <pack>` / `deps <pack>`: shows composed packs, referenced capabilities, sources, and trust.
- `publish --check`: runs contributor checks before opening a registry PR.
- `scan [path]`: discovers existing `SKILL.md` files.
- `import <skills-dir>`: copies discovered skills into `<target>/sources/imported/`.
- `lint <pack>`: validates pack metadata.
- `verify <pack>`: expands a pack, checks duplicate/missing capability sources, and warns about moving remote refs.
- `resolve <pack>`: classifies sources as local, GitHub tree, pinned, or moving refs.
- `policy check <pack> <policy.json|preset>`: enforces allow/deny source rules, pinned refs, and native-command policy. Built-in presets live under `registry/policy/`, for example `default`, `ci`, and `strict`.
- `licenses <pack>` and `attribution <pack>`: report license and source attribution.
- `index [--output path]`: generates a searchable registry index.
- `diff <pack>`: compares the installed lockfile with the current registry pack.
- `pin <pack> [--check]`: resolves each installed capability's moving ref to a concrete commit and content checksum and records them in the lockfile; `--check` re-resolves the live sources and fails if any drifted from the recorded pins.
- `compat <pack> --agent <tool>`: checks tool compatibility metadata.
- `cache prune|clean`: removes cached state; `clean` also removes imported sources.

## Remote Registries

Registries are named sources stored in `<target>/registries.json`.

```sh
cli/bin/agent-packs registry add official https://github.com/agent-packs/registry --target ~/.agent-packs
cli/bin/agent-packs registry list --target ~/.agent-packs
cli/bin/agent-packs install official/frontend-engineer --target ~/.agent-packs
cli/bin/agent-packs registry remove official --target ~/.agent-packs
```

A registry source can be a local repository path or a Git URL. Remote registries are cloned into `<target>/registries/<name>/` and resolved from either `registry/packs/` or `packs/`.

## Specifying Plugins And Skills

Plugins and skills are declared as entries in `capabilities`. Plugin entries must include `format` and `install` metadata so an installer can resolve the marketplace/package/command. Skill entries must include `format` and `entry` so an installer can locate the `SKILL.md` file. Any capability can include optional `upstreamSource` when separate provenance metadata is useful.

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
    "command": "claude plugin install code-review@claude-plugins-official",
    "uninstall": "claude plugin uninstall code-review@claude-plugins-official"
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

Packs can include other packs with the `packs` field. They can also include reusable source references with `skills` and `plugins`. `skills` and `plugins` entries can be registry ID strings or objects with their own remote `source`. Included packs and referenced capabilities are expanded before install.

```json
{
  "id": "review-combo",
  "name": "Review Combo Pack",
  "version": "0.1.0",
  "description": "Composes review-oriented packs.",
  "packs": ["pr-review"],
  "skills": [
    "frontend-implementation-guidance",
    {
      "id": "remote-planning-skill",
      "source": "https://github.com/addyosmani/agent-skills/tree/main/skills/planning-and-task-breakdown",
      "format": "agent-skill",
      "entry": "SKILL.md"
    }
  ],
  "plugins": [
    "browser-verification-workflow",
    {
      "id": "remote-code-review-plugin",
      "source": "https://github.com/anthropics/claude-plugins-official/tree/main/plugins/code-review",
      "format": "anthropic-plugin",
      "entry": ".claude-plugin/plugin.json"
    }
  ]
}
```

Reusable skills live as Agent Skills at `skills/<id>/SKILL.md` (in the registry repo). Reusable plugins live as Claude Code plugins at `plugins/<id>/.claude-plugin/plugin.json`. A pack can reference them by ID, or bypass local registry entries by using object refs with remote `source` URLs. The CLI treats both forms as references rather than installable copies.

Agent Skills follow the Agent Skills specification: a skill directory with required `SKILL.md` frontmatter fields `name` and `description`. Claude Code plugins follow the plugin manifest layout with `.claude-plugin/plugin.json` and a required `name` field. Use `metadata.agentpacks.source` on registry skills and `repository` or `homepage` on registry plugins to point at the remote source.

Registry entries can include catalog metadata such as `maintainers`, `stability`,
`lastVerified`, `reviewStatus`, `deprecated`, `replacement`, and `requirements`.
These fields power registry search, publish checks, and the static catalog.

## Examples

Example manifests live in the registry repo under `schemas/examples/`:

- `minimal-pack.json`: the smallest valid pack manifest.
- `full-pack.json`: a complete manifest showing every supported capability type.
- `real-world-pack.json`: examples based on public Claude Code plugin and Agent Skills repositories.
- `composed-pack.json`: a pack that includes another pack.
- `referenced-capabilities-pack.json`: a pack that includes reusable `skills` and `plugins` entries.

## Tests

```sh
cd cli && go test ./...
python3 -m unittest discover -s tests
```

## Core Concepts

- Pack: a curated bundle for a role, stack, workflow, or task.
- Categories/tools/scope: searchable facets for registry discovery and install intent.
- Skill: an instruction module, often `SKILL.md`.
- Plugin: a packaged agent extension, such as an Anthropic/Claude Code plugin.
- Tool: shell command, API connector, or executable integration.
- Recipe: recommended combinations of packs for a larger use case.

## License

Apache-2.0
