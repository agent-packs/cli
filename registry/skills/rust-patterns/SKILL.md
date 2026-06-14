---
name: rust-patterns
description: Write idiomatic, correct Rust using ownership, lifetimes, trait design, async patterns, and safe abstractions over unsafe code. Use when building Rust libraries, services, or systems code.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Rust Patterns

The borrow checker is a collaborator, not an obstacle. When it fights you, reconsider the design.

## Ownership and Borrowing

- Prefer passing by reference (`&T`, `&mut T`) over cloning unless the callee needs ownership. Cloning to satisfy the borrow checker is usually a design smell.
- Return owned values from constructors and factory functions; accept references in methods that only inspect.
- When multiple parts of a struct need to be borrowed mutably at the same time, split the struct or introduce a helper method that borrows them together in one `&mut self` call.
- Avoid `Rc<RefCell<T>>` in hot paths — it trades compile-time safety for runtime panics and adds indirection. Restructure data to have a single owner instead.

## Lifetimes

- Elide lifetimes where the compiler can infer them (one input reference, or `&self`/`&mut self`). Add explicit annotations only when the compiler requires them or when they document an important relationship.
- When a struct holds a reference, the lifetime annotation on the struct expresses "this struct cannot outlive the data it borrows." Use `Arc<T>` or owned data instead when that constraint is too restrictive.
- Avoid `'static` bounds unless you genuinely need the data to live forever (thread spawning, global caches). `'static` in trait bounds often signals an architecture that should use `Arc` instead.

## Error Handling

- Use `thiserror` for library crates (derives `std::error::Error` with clean display) and `anyhow` for application crates (adds context with `.context("while doing X")`).
- Never use `.unwrap()` or `.expect()` in library code — callers cannot handle a panic. In application code, `.expect("invariant: X is always set at startup")` is acceptable at program boundaries.
- Use the `?` operator uniformly. A function that mixes `?` and `match` for error handling is inconsistent — pick one style per function.
- Represent domain errors as enums with variants, not strings. `AuthError::TokenExpired` is matchable; `"token expired"` is not.

## Trait Design

- Implement standard traits where they make semantic sense: `Display` for human output, `Debug` always, `Clone` only if copying is cheap and meaningful, `PartialEq`/`Eq` for value types.
- Keep traits small and focused. A trait with more than 3–5 methods is likely mixing concerns — split it. Blanket implementations become impossible when traits are fat.
- Prefer `impl Trait` in function signatures over generic type parameters when the concrete type doesn't need to be named by the caller: `fn process(items: impl Iterator<Item = u32>)`.
- Use `Into<T>` for constructor arguments that accept multiple types: `fn new(name: impl Into<String>)`. It removes boilerplate at call sites without sacrificing type safety.

## Async

- Use `tokio` as the default async runtime. Don't mix runtimes in a single binary.
- Never block inside an async function: no `std::thread::sleep`, no synchronous I/O, no `Mutex::lock` that might block for more than microseconds. Use `tokio::time::sleep`, async I/O, and `tokio::sync::Mutex`.
- Use `tokio::spawn` for truly independent tasks; use `.await` for sequential or dependent work. Over-spawning adds context-switching overhead.
- `select!` is for racing multiple futures; don't use it as a workaround for non-cancellable futures — it drops the losing branch, which may leave resources in an inconsistent state.

## Unsafe

- Every `unsafe` block requires a `// SAFETY:` comment explaining why the invariants that make this safe are upheld.
- Minimize the surface of unsafe code: wrap it in a safe abstraction immediately. The unsafe impl should be a small, auditable function, not spread across the codebase.
- Run `cargo miri` on unsafe code to catch undefined behavior that the compiler cannot detect.
- Prefer `unsafe` in a dedicated module with a module-level safety contract over scattered `unsafe` blocks.

## Checklist

- [ ] No `.unwrap()` in library crate paths that callers traverse.
- [ ] Error types use `thiserror` (library) or `anyhow` (application).
- [ ] Every `unsafe` block has a `// SAFETY:` comment.
- [ ] No blocking calls inside async functions.
- [ ] Clippy passes with `cargo clippy -- -D warnings`.
- [ ] `cargo miri test` run on any crate containing `unsafe`.
