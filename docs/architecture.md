# Architecture

Agent Packs should feel like Homebrew for agent capabilities.

## Production Stack

The recommended production implementation is:

- Go CLI named `agent-packs`, with short alias `ap`.
- Static registry index hosted on GitHub Pages, S3, Cloudflare R2, or a CDN.
- Pack manifests as JSON or YAML, validated against `schemas/agent-pack.schema.json`.
- Install receipts stored under the user's agent configuration directory.
- Optional TypeScript web app for discovery, ratings, and pack pages.

## CLI Commands

Initial commands:

- `ap search [query]`
- `ap show <pack>`
- `ap install <pack>`
- `ap uninstall <pack>`
- `ap list`
- `ap update`
- `ap doctor`

## Install Experience

Target install experience:

```sh
brew install agent-packs
ap install frontend-engineer
```

Bootstrap fallback:

```sh
curl -fsSL https://agentpacks.dev/install.sh | sh
```

## Why Go

Go is the safest default for a brew-like developer tool:

- Single binary distribution.
- Fast startup.
- Good filesystem and archive support.
- Easy release automation with GitHub Actions.
- Clean cross-compilation.
- Familiar enough for infrastructure-oriented contributors.

Rust is also a strong choice, especially if the CLI needs maximum performance or
deep package-management guarantees. It has a steeper contribution curve. Node is
good for rapid iteration and web-adjacent contributors, but less ideal for a
system installer because users inherit runtime and package-manager complexity.

