---
name: onboarding-and-archaeology
description: Ramp up quickly on an unfamiliar codebase by reading strategically, mapping invariants, and making safe first changes. Use when joining a project, reviewing a legacy system, or orienting in a large monorepo.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Onboarding and Code Archaeology

An unfamiliar codebase is an archaeological site. Read the strata before you dig.

## Orientation Pass (First Hour)

- Start with the entry point: `main`, `index`, `app.py`, `cmd/`. Follow the startup sequence to understand what the process does before it handles any request.
- Read `CLAUDE.md`, `CONTRIBUTING.md`, `README.md`, and any `docs/` folder. These record decisions that are no longer visible in the code.
- Check the CI definition (`.github/workflows/`, `.circleci/`, `Makefile`). The build steps tell you what the maintainers consider essential.
- `git log --oneline -50` gives you recent momentum: what changed, at what frequency, and by whom.
- `git log --oneline --follow -- <file>` on the most-changed file shows what's been fought over.

## Mapping the System

- Draw a box diagram of the major components from `git log` and directory structure before reading any code deeply. Hypothesize, then verify.
- Identify the seams: where do modules call each other? Grep for import statements and function call sites to find the dependency graph.
- Find the data model first — it's the stable substrate everything else is built on. Schema files, ORM models, and proto definitions are high-leverage reads.
- Identify the top-level invariants: what must always be true? Look for comments marked `// INVARIANT`, `// IMPORTANT`, `// NOTE`, panics, and assertions — they mark the load-bearing assumptions.

## Reading Code Strategically

- Read tests before implementation. Tests document intended behavior and reveal edge cases the author thought about. Missing tests reveal gaps.
- Read the error paths as carefully as the happy path. Error handling often contains the most domain knowledge.
- When a function is surprising, read its git history (`git log -p -- <file>`): the patch context explains *why*, not just *what*.
- Don't read everything. Spend the first day identifying which 20% of the code is responsible for 80% of the behavior — focus there.

## Safe First Changes

- Before touching anything, add a test for the behavior you're about to change. If no test exists, write one that passes today. This is your safety net.
- Make the smallest possible change first: a one-line fix, a renamed variable, a new test. Running the full test suite on a trivial change validates your local setup.
- Prefer additive changes (new function, new flag) over modifications. Modifications break callers you haven't found yet.
- When changing behavior, use the expand-contract pattern: add the new path, migrate callers, delete the old path — three separate, reviewable steps.
- Flag your assumptions in PR descriptions: "I believe X is always true here because Y. If that's wrong, this change will break Z."

## Asking the Right Questions

- Don't ask "what does this code do?" — read it. Ask "why does it do it this way?" and "what would break if it did it differently?"
- When something looks wrong, assume you're missing context before assuming it's a bug. Check git blame, linked issues, and ADRs first.
- Document what you learn in the code or in ADRs as you go — future contributors (including you in 6 months) will have the same questions.

## Checklist

- [ ] Entry point and startup sequence read.
- [ ] Data model and top-level invariants identified.
- [ ] `git log` reviewed for recent momentum and hotspots.
- [ ] Tests read before implementation for each area being modified.
- [ ] First change covered by a test before modifying code.
- [ ] Surprising code traced through git history for context.
