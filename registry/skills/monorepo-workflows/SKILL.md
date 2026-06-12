---
name: monorepo-workflows
description: Manage monorepos efficiently with consistent build tooling, scoped changes, and dependency hygiene. Use when working in or setting up a monorepo with multiple packages or applications.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.2.0"
---

# Monorepo Workflows

A monorepo scales when tooling enforces boundaries and CI only builds what changed.

## Tooling Choice

- **Turborepo** — JS/TS, simple config, excellent remote caching. Best default for most teams.
- **Nx** — JS/TS/Go/Java, opinionated generator-based workflow, project graph visualization.
- **Bazel** — Polyglot, deterministic builds, scales to very large repos. High setup cost.
- **Pants** — Python/Java/Go, fine-grained dependency tracking.
- Choose one orchestrator; mixing two creates conflicting task graphs.

## Version Policy

- **Lockstep**: single root `package.json` + unified lockfile. All packages share one version. Simple but forces coordinated releases.
- **Independent**: each package owns its version and changelog. Requires a release tool (Changesets, semantic-release, Nx Release).
- Never mix both strategies in the same repo — it creates invisible coupling.

## Task Graph and Caching

- Define the task dependency graph explicitly: `"dependsOn": ["^build"]` in Turborepo, `targetDefaults` in Nx.
- Enable remote caching in CI to share artifacts across branches and runners.
- Mark tasks `cache: false` only for true side effects: deploy, publish, external API calls.
- Validate caching: run `turbo build` twice; the second run must be 100% cached with no outputs re-executed.

## Scoped Changes

- Before starting any change, identify affected packages: `turbo run build --filter=...[HEAD^1]` or `nx affected`.
- Never change a shared package without running tests for all its dependents.
- Use changesets to manage version bumps: one changeset file per logical change, merged with the PR.

## Workspace Hygiene

- Hoist shared devDependencies to the root. Only keep runtime deps that differ per package local.
- Enforce internal import paths with ESLint (`import/no-internal-modules`) or TypeScript path aliases — no reaching into a sibling's `src/` directly.
- Ban circular dependencies: run `madge --circular` or the Nx graph checker in CI on every PR.
- Keep workspace config (`pnpm-workspace.yaml`, `workspaces` in `package.json`) in sync with actual package directories.

## CI Strategy

- PRs: build and test only affected packages using the git diff range filter.
- Trunk merges: build and test all packages.
- Run linting and type-checking before expensive build and test steps (fail fast).
- Cache node_modules and build artifacts between runs using the remote cache.

## Checklist

- [ ] Task graph is defined with `"dependsOn": ["^build"]` for `build` tasks.
- [ ] Remote cache is configured and verified (second run is 100% cached).
- [ ] No circular dependencies between packages.
- [ ] Changeset or release tool configured for versioning.
- [ ] CI runs affected-only on PRs and full build on trunk.
- [ ] No cross-package `src/` imports — only through declared exports.
