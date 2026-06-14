---
name: dependency-management
description: Keep dependencies secure, minimal, and up to date. Use when auditing a project's supply chain, adding new libraries, evaluating upgrades, or hardening a CI pipeline against dependency risks.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Dependency Management

Every dependency is code you didn't write, can't fully review, and must maintain forever. Choose carefully.

## Evaluating New Dependencies

Before adding a dependency, ask:
- Can I implement this in under 100 lines without the library? If yes, consider doing so.
- Is the library actively maintained? (Last release date, open issue count, PR response time)
- How many transitive dependencies does it add? (`npm why`, `cargo tree`, `pip show`)
- Is the license compatible with this project?
- Has it had security advisories in the past 12 months, and how were they handled?

Prefer dependencies that do one thing well over ones that solve everything.

## Lock Files

- Commit lock files (`package-lock.json`, `Cargo.lock`, `poetry.lock`, `go.sum`) for applications. Lock files guarantee reproducible builds.
- For libraries, commit `Cargo.lock` (Rust) but do not commit `package-lock.json` (npm) — downstream consumers resolve their own trees.
- Never edit lock files by hand. Regenerate them by upgrading the source manifest and re-running the package manager.
- Treat lock file diffs in PRs as code — review them. Unexpected transitive version changes are a supply chain risk.

## Keeping Dependencies Updated

- Use Dependabot, Renovate, or a similar bot to open automatic upgrade PRs. Configure it with a schedule (weekly for minor/patch, manual for major) and a reviewer assignment.
- Group patch updates into a single PR; review minor and major updates individually.
- Read changelogs for every minor and major upgrade before merging. Don't auto-merge without a changelog review.
- Prefer upgrading one dependency at a time so regressions are attributable.

## Security Auditing

- Run dependency audits in CI on every merge: `npm audit`, `cargo audit`, `pip-audit`, `govulncheck`.
- Set audit to fail CI on `high` or `critical` severities. `moderate` should be tracked and addressed within a sprint.
- Subscribe to security advisories for your major dependencies via GitHub Advisory Database or the relevant language ecosystem's advisory feed.
- Audit licenses in CI with `license-checker` (npm) or `cargo-deny` (Rust). An undiscovered GPL transitive dependency can create legal obligations.

## Pinning and Reproducibility

- Pin exact versions in production application manifests: `"react": "18.2.0"` not `"react": "^18"`. Floating ranges shift the build on every install.
- For Docker base images, pin to a digest (`node@sha256:...`) not just a tag. Tags are mutable.
- For Go, `go mod tidy` removes unused dependencies — run it before every release.
- For Python, use `pip-compile` (pip-tools) or `poetry lock` to generate a pinned requirements file from a high-level spec.

## Reducing the Footprint

- Periodically audit for unused dependencies: `npm-check`, `cargo machete`, `deptry` (Python). Remove anything not actively used.
- Prefer `devDependencies` over `dependencies` for build and test tools — they don't ship to production.
- When a dependency is used in only one place, evaluate whether the function can be inlined and the dep removed.

## Checklist

- [ ] New dependencies evaluated for maintenance health, license, and transitive footprint.
- [ ] Lock file committed and reviewed in PRs.
- [ ] Dependency audit running in CI, failing on high/critical CVEs.
- [ ] Automated upgrade bot configured with changelog review requirement.
- [ ] License audit in CI for production application.
- [ ] Unused dependencies pruned on a quarterly schedule.
