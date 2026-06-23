# Spec: Capability authoring parity, smart init, and hook safety

Status: **Implemented** (Phase 4) — Phases A, B, C landed; feature #4 deferred
Owner: CLI maintainers
Target: `agent-packs/cli` (Go). Registry content lives in the separate
`agent-packs/registry` repo and is out of scope here.

## Objective

v0.6.0 shipped four new capability types (`command`, `hook`, `memory`,
`settings`) into the install/plan/drift engine, but the surrounding CLI did not
catch up. This spec closes that gap across three features, delivered as
**sequenced phases** so each ships independently as a fast-follow.

| # | Feature | Problem it solves | Phase |
|---|---------|-------------------|-------|
| 1 | Authoring parity | `agent-packs new` only scaffolds `pack\|skill\|plugin`; there is no on-ramp to author the new capability types, so the registry stays skill/plugin-heavy and the new engine support goes unused. | A (first) |
| 3 | Hook safety & preview | Installing a `hook` writes a file the target agent will execute automatically. Today that happens with no preview and no explicit opt-in — inconsistent with the `--execute-plugins` gate. | B |
| 2 | Smart `init` onboarding | `agent-packs init` writes a static config from flags. New users must already know which agents/packs they want. | C |

Cross-agent capability portability (feature #4) is deferred — see
[Future phase](#future-phase-deferred).

### Users
- **Pack authors** (feature 1): contributors writing manifests in a checkout of
  `agent-packs/registry`.
- **End users** (features 2, 3): engineers installing packs into their agents.

### Success looks like
- An author can scaffold a valid `command`/`hook`/`memory`/`settings` capability
  with one command and `validate` passes on the output.
- Installing a hook is preview-able and requires explicit consent, mirroring
  plugin execution.
- A new user runs `agent-packs init` in a repo and gets a config pre-populated
  with detected agents and recommended packs, with zero prior knowledge.

## Tech Stack
- Go (module `github.com/agent-packs/cli`, toolchain per `go.mod`, currently 1.26).
- Existing internal packages only — no new runtime dependencies expected.
- YAML config via the dependency already used in `internal/config`.
- Python `unittest` for integration tests under `tests/`.

## Commands
```sh
# Build
go build -o bin/agent-packs ./cmd/agent-packs

# Go unit tests (all / single package)
go test ./...
go test ./internal/author/... ./internal/config/... ./internal/install/...

# Python integration + docs tests (venv already present at .venv)
.venv/bin/python -m unittest discover -s tests

# Validate scaffolded output against the registry schema
bin/agent-packs validate /tmp/registry/packs        # path form
AGENT_PACKS_REGISTRY=/tmp/registry/packs bin/agent-packs lint   # registry form
```

## Project Structure
Changes stay within the existing layered packages (`model` → `targets`/`registry`
→ `plan` → `install` → `agentpacks` → `cmd`).

```
cmd/agent-packs/main.go        → flag parsing for `new`, `init`, `install`; usage/completions
internal/author/author.go      → New() scaffolding — extend kind switch (feature 1)
internal/validate/validate.go  → per-capability validation/lint (feature 1)
internal/install/install.go    → installManagedFile + hook gating (feature 3)
internal/plan/plan.go          → plan/dry-run output for hooks (feature 3)
internal/config/config.go      → ProjectConfig + Init() (feature 2)
internal/targets/targets.go    → agent detection helpers (feature 2)
internal/agentpacks/           → command wiring (New, InitProject, detection)
docs/specs/                     → this spec (committed)
tests/                          → Python integration (test_install.py, new test_init.py)
```

## Code Style
Match existing Go: small focused functions, explicit error wrapping, table-driven
tests. New scaffolding follows the shape already in `author.go`:

```go
func newCommand(opts NewOptions) (string, error) {
	cap := model.Capability{
		Type:    "command",
		Name:    opts.Name,
		Format:  "markdown",
		Content: "Describe the prompt this command runs.",
	}
	path := filepath.Join(opts.Dir, opts.ID+".json")
	return path, writeJSON(path, cap, opts.Force)
}
```

Conventions: kebab-case capability IDs; `Slugify` for derived filenames;
sentence-case description placeholders; preview-safe defaults (no execution, no
network) unless an explicit flag opts in.

## Testing Strategy
- **Unit (Go), co-located `_test.go`:**
  - `internal/author`: each new kind scaffolds a manifest that `validate`
    accepts; `--force` and exists-guard behavior preserved.
  - `internal/validate`: new-kind manifests validate; malformed ones produce the
    expected error strings.
  - `internal/install`: hook install is blocked without the consent flag and
    succeeds with it; dry-run surfaces hook content without writing.
  - `internal/config` + detection: stack/agent detection from fixture dirs
    yields the expected recommended packs and config.
- **Integration (Python `tests/`):** extend `test_install.py` for the hook
  consent gate; add `test_init.py` for smart-init detection against temp repos
  (set `AGENT_PACKS_REGISTRY`, no network).
- **Docs:** `test_docs.py` invariants must stay green; update README/usage and
  shell completions for new `new` kinds and any new install flag.
- Coverage expectation: every new branch (each new kind, the gate on/off, each
  detector) has at least one test. No net-new untested public function.

## Boundaries
- **Always:** run `go test ./...` and the Python suite before committing; run
  `validate`/`lint` against a registry checkout for scaffolding changes; keep the
  layered package dependency direction; keep new behavior preview-safe by default.
- **Ask first:** any change to the `agent-packs/registry` JSON schema; adding a
  runtime dependency; changing the default behavior or output of existing
  commands (`init`, `install`, `new`); changing the `.agent-packs.yaml` format;
  changing trust/policy defaults.
- **Never:** install or execute hooks/plugins without an explicit opt-in flag;
  author real registry content as part of this CLI work; commit secrets; weaken
  existing trust-tier or policy enforcement; remove or skip failing tests without
  approval.

## Feature detail & success criteria

### Phase A — Feature 1: Authoring parity
- Extend `author.New` kind switch and `runNew` usage/validation to accept
  `command`, `hook`, `memory`, `settings`. Each emits a minimal valid manifest
  (inline `content` by default; a commented `source` alternative).
- Update `validate`/`lint` so scaffolded output passes and bad input gives clear
  messages (reuse `capabilityAllowsInlineContent`).
- Update README `new pack|skill|plugin` references, `usage()`, and bash/zsh
  completions to list the new kinds.
- **Done when:** `for k in command hook memory settings; do agent-packs new $k demo-$k --dir /tmp/x; done`
  produces files that `validate` accepts, and tests cover each kind. README/usage
  and completions list all kinds; `test_docs.py` stays green.

### Phase B — Feature 3: Hook safety & preview
- In `--mode copy`, writing a `hook` file requires explicit consent via a new
  `--allow-hooks` flag (parallel to `--execute-plugins`). Without it, hooks are
  recorded/skipped with a clear reason, not written.
- `--dry-run` (and the recorded plan) surfaces the resolved hook destination and
  a content preview so the user sees what the agent will execute.
- Drift detection for hooks already exists (content hash) and must keep passing.
- **Done when:** `install ... --mode copy` does **not** write a hook file unless
  `--allow-hooks` is passed; `--dry-run` prints hook content/destination;
  existing hook drift/uninstall tests still pass; new tests cover gate on/off.

### Phase C — Feature 2: Smart init
- `agent-packs init` gains detection: agents from on-disk presence (existing
  target matrix paths) and stack from manifest files (`package.json`, `go.mod`,
  `Cargo.toml`, `pyproject.toml`). It maps detections to recommended pack IDs via
  registry search and writes them into `ProjectConfig.Packs` and `.Agent`.
- Detection is non-destructive and offline-relative-to-the-target: it reads the
  project dir and the (already-fetched) registry; it never installs or executes.
- Add `--no-detect` to fall back to today's flag-only behavior; keep `--force`
  semantics. The exists-guard on `.agent-packs.yaml` is preserved.
- **Done when:** running `init` in a fixture repo with a `go.mod` and a present
  `.claude/` dir writes a config naming `claude` and at least one relevant
  recommended pack; `--no-detect` reproduces current output; tests cover each
  detector and the no-detect path.

## Future phase (deferred)
**Feature 4 — Cross-agent capability portability.** Translate a capability
authored for one agent (e.g. a Claude `.claude/commands/*.md` command) into the
portable `.agent-packs/...` form or another agent's native path, building on the
existing `agentTargets` overrides and portable fallbacks. Large and research-
heavy; to be specified separately once authoring volume (Phase A) justifies it.
A safe v1 would be one direction only (native → portable) with explicit review.

## Resolved decisions
_All four open questions resolved during spec review:_
1. **`new` output target:** emit a **standalone capability JSON** for every new
   kind, matching the existing `new pack|skill|plugin` flow. Authors paste it
   into a pack's `capabilities[]` or reference it.
2. **Hook gate flag:** a distinct **`--allow-hooks`** flag (not a broader
   `--allow-execution` umbrella), keeping the hook blast radius explicit and
   parallel to `--execute-plugins`.
3. **Recommendation source for `init`:** **tag/category matching** of registry
   packs against the detected stack. No curated stack→pack table; revisit only
   if match precision proves poor.
4. **Agent detection scope:** **project-local only** for `init` (reads the
   project dir). Global detection (`~/.claude`, etc.) is out of scope here and
   could later live in `doctor`.

---

## Implementation Plan (Phase 2)

### Components & dependency order
The three phases are independent and could ship in any order, but A is
sequenced first because it is the smallest change and unblocks authoring of the
content the other features surface. Within each phase, work flows bottom-up
through the layered packages.

**Phase A — Authoring parity** (`internal/author` → `cmd` → docs)
1. `internal/author/author.go`: add `newCommand/newHook/newMemory/newSettings`,
   extend the `New` kind switch. Reuse `writeJSON` and `Slugify`.
2. `cmd/agent-packs/main.go`: widen `runNew` usage string + kind validation;
   update `usage()` and the bash/zsh completion kind lists.
3. `internal/validate`: confirm scaffolded output passes; add targeted
   error-message coverage (no schema change — types already in the enum).
4. README: update the `new pack|skill|plugin` references.

**Phase B — Hook safety** (`cmd`/`plan`/`install`)
1. Thread a new `allowHooks bool` through `model.InstallOptions` (or the install
   call path) — find where `executePlugins` flows and mirror it.
2. `internal/install/install.go`: in `installManagedFile`, when `item.Type ==
   "hook"` and not allowed, set `recorded` with a clear reason instead of
   writing. Gate only hooks, not commands.
3. `internal/plan` / dry-run output: include hook destination + a content
   preview line.
4. `cmd/agent-packs/main.go`: add `--allow-hooks` flag, usage, completions.

**Phase C — Smart init** (`internal/targets`/`registry` → `config`/`agentpacks`
→ `cmd`)
1. Detection helpers: `detectAgents(projectDir)` (probe target-matrix project
   paths) and `detectStack(projectDir)` (manifest files → tags).
2. Recommendation: query registry packs by tag/category overlap with detected
   stack; cap and rank results.
3. `internal/config` + `agentpacks.InitProject`: populate `ProjectConfig.Agent`
   and `.Packs` from detection; add `--no-detect`; preserve exists-guard/`--force`.
4. `cmd`: wire flag + usage; print a short "detected … / recommending …" summary.

### Parallelism
- A, B, C touch mostly disjoint files and can proceed in parallel if needed.
  The only shared file is `cmd/agent-packs/main.go` (flags/usage/completions) —
  serialize edits there or expect trivial merge resolution.

### Risks & mitigations
| Risk | Mitigation |
|------|------------|
| Hook gate changes default `install` behavior (hooks silently stop writing) | Loud `recorded` reason in plan output + CHANGELOG note; only affects `--mode copy` hooks, which are new in v0.6.0 (low installed base). |
| Tag-based recommendations are noisy/empty | Rank by overlap count, cap to N, and fall back to flag-only config when no confident match; `--no-detect` always available. |
| Detection false-positives on agents | Probe concrete matrix paths only; treat absence as "not detected" rather than guessing. |
| `cmd/main.go` flag/usage drift vs. completions | `test_docs.py`-style guard or a usage/completions check; update all three sites in the same commit. |
| Scaffolded JSON drifts from registry schema | Each phase-A test runs `validate` on generated output, catching enum/field drift against the live schema. |

### Verification checkpoints (gate between phases)
- **After A:** `for k in command hook memory settings; do bin/agent-packs new $k demo-$k --dir /tmp/x; done` emits clean standalone capability JSON; `go test ./internal/author/...` proves each scaffold passes `validate.ValidateCapability` (the pack-level `validate` command validates pack manifests, not standalone capability files — a scaffolded capability is validated once pasted into a pack's `capabilities[]`); README/usage/completions list new kinds; `go test ./...` + Python suite green.
- **After B:** install of a hook in `--mode copy` writes nothing without `--allow-hooks` and writes with it; `--dry-run` shows the preview; existing hook drift/uninstall tests pass; new gate tests pass.
- **After C:** `init` in a Go+`.claude` fixture writes `agent: claude` and ≥1 recommended pack; `--no-detect` matches today's output; detector tests pass; full suite green.
- Each checkpoint ends with CI green on its PR before the next phase starts.
