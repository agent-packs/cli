# Agent Packs

Curated, installable capability bundles for AI coding agents.

Agent Packs bundles public Skills, Plugins, MCP servers, commands, hooks, prompts,
and templates into ready-to-use workflow packs.

## Language Recommendation

Use Go for the production CLI.

Go is the best fit for a Homebrew-like installer because it produces small
single-file binaries, cross-compiles cleanly for macOS/Linux/Windows, starts
quickly, and does not require users to install Node, Python, or Rust first.

Recommended stack:

- CLI: Go
- Registry metadata: JSON documents validated by JSON Schema
- Pack manifests: YAML or JSON
- Web/API registry: TypeScript later, if needed
- Install scripts: POSIX shell for macOS/Linux bootstrap

This repository starts with a dependency-free Python prototype so the product
model can be tested immediately in this workspace. The registry and manifest
formats are intentionally language-neutral.

## Prototype Usage

```sh
python3 dev/bin/agent-packs search
python3 dev/bin/agent-packs show frontend-engineer
python3 dev/bin/agent-packs install frontend-engineer --target ./sandbox
```

## Examples

Example manifests live in `dev/schemas/examples/`:

- `minimal-pack.json`: the smallest valid pack manifest.
- `full-pack.json`: a complete manifest showing every supported capability type.

## Tests

```sh
python3 -m unittest discover -s dev/tests
```

## Core Concepts

- Pack: a curated bundle for a role, stack, workflow, or task.
- Skill: an instruction module, often `SKILL.md`.
- Plugin: a packaged agent extension, such as an Anthropic/Claude Code plugin.
- Tool: MCP server, shell command, API connector, or executable integration.
- Recipe: recommended combinations of packs for a larger use case.

## License

Apache-2.0
