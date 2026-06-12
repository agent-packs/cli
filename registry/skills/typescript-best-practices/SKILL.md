---
name: typescript-best-practices
description: Write type-safe TypeScript using strict settings, precise types, and disciplined migration patterns. Use when writing TS, reviewing types, or migrating from JavaScript.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.2.0"
---

# TypeScript Best Practices

Types are documentation that the compiler checks. Write types that reveal intent, not types that silence the compiler.

## Compiler Settings

Always use `"strict": true` in `tsconfig.json`. Additionally enable:
- `"noUncheckedIndexedAccess": true` — array and object index access returns `T | undefined`.
- `"exactOptionalPropertyTypes": true` — `{ a?: string }` means absent, not `string | undefined`.
- `"noImplicitReturns": true` and `"noFallthroughCasesInSwitch": true`.
- `"moduleResolution": "bundler"` (or `"node16"` / `"nodenext"`) for modern module semantics.

## Type Precision

- Prefer `unknown` over `any` for values of unknown shape; narrow with type guards before use.
- Use `const` assertions (`as const`) to preserve literal types in arrays and objects.
- Model discriminated unions over class hierarchies for state machines and response variants.
- Avoid `!` non-null assertions; prefer explicit narrowing or nullish coalescing (`??`).
- Use `satisfies` to validate a value against a type without widening the inferred type.
- Prefer `readonly` arrays and properties in function signatures; mutate via explicit copies.

## Functions and Generics

- Annotate return types explicitly on all exported functions to prevent accidental widening.
- Constrain generics minimally: `<T extends string>` not `<T>` when you only need string operations.
- Prefer function overloads over union parameter types when the return type changes with input shape.
- Use `Parameters<typeof fn>`, `ReturnType<typeof fn>`, and `Awaited<T>` over duplicating types.

## Narrowing and Type Guards

- Use `typeof`, `instanceof`, `in`, and discriminant property checks — not `as` casts.
- Write user-defined type guards (`value is Type`) only when built-in narrowing cannot express the check.
- Use exhaustiveness checks: `default: { const _: never = x; throw new Error('unhandled case'); }`.
- Never use `as unknown as TargetType` as a shortcut — it hides real type errors.

## JavaScript Migration

1. Add `"allowJs": true` and `"checkJs": true` — fix reported errors without renaming files.
2. Rename files leaf-first: modules with no imports of other JS files first, then work up the dependency graph.
3. Generate `.d.ts` stubs for third-party modules missing types before adding them to the graph.
4. Replace `@ts-ignore` with `@ts-expect-error` plus a reason comment; delete when fixed.
5. Enable `strict` only after the codebase is fully `.ts` — tackle one error category at a time.

## Code Review Checklist

- [ ] No `any` in new or changed code — use `unknown` + narrowing or a named type.
- [ ] All exported API surfaces have explicit return type annotations.
- [ ] Discriminated unions cover all cases with exhaustiveness checks.
- [ ] `"strict": true` in tsconfig; no new `skipLibCheck` suppressions.
- [ ] No `as` casts that skip validation; use type guards instead.
