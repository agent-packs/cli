# Architecture

Agent Packs should feel like Homebrew for agent capabilities while keeping the CLI and registry separable.

## Production Stack

- CLI: Go module under `cli/`.
- Registry: static JSON manifests under `registry/packs/`.
- Schema: `registry/schemas/agent-pack.schema.json`.
- Receipts: `<target>/receipts/<pack-id>.json`.
- Lockfiles: `<target>/packs/<pack-id>/agent-pack.lock`.

## CLI Commands

Implemented commands:

- `agent-packs search [query]`
- `agent-packs show <pack>`
- `agent-packs install <pack|registry/pack>`
- `agent-packs list`
- `agent-packs uninstall <pack>`
- `agent-packs doctor`
- `agent-packs validate <file-or-directory>`
- `agent-packs registry add <name> <source>`
- `agent-packs registry list`
- `agent-packs registry remove <name>`

## Install Experience

Target install experience:

```sh
brew install agent-packs
agent-packs install frontend-engineer
```

Bootstrap fallback:

```sh
curl -fsSL https://agentpacks.dev/install.sh | sh
```

## Security Posture

Plugin install commands are not executed unless the user passes `--execute-plugins`. Plugin capabilities with install commands should set `requiresExecution: true` and should include trust metadata such as `trust: "official"` or `trust: "community"`.

Remote skills are fetched with `git`, copied into the selected agent target, and recorded in receipts. Integrity metadata can be represented with `integrity.checksum` and `integrity.signature`; lockfiles record a digest for every capability.

## Why Go

Go is the safest default for a brew-like developer tool:

- Single binary distribution.
- Fast startup.
- Good filesystem and archive support.
- Easy release automation with GitHub Actions.
- Clean cross-compilation.
- Familiar enough for infrastructure-oriented contributors.
