---
name: technical-writing
description: Write clear, accurate, and maintainable developer documentation — API references, tutorials, guides, and changelogs. Use when documenting APIs, onboarding new contributors, or creating user-facing technical content.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Technical Writing

Documentation is a product. It has users, it has bugs, and it goes stale just like code.

## Audience and Tone

- Name your audience before you write: "This guide is for backend engineers who haven't worked with the auth service before." Write to that person, not to everyone.
- Use second person ("you") for instructional content. Reserve "we" for announcements and release notes.
- Prefer short, declarative sentences. A 40-word sentence hides the instruction inside the prose.
- Avoid hedging ("might", "could", "sometimes", "in some cases") in procedural docs. Say what will happen.
- No jargon without definition on first use. Link acronyms to a glossary.

## API Reference

- Document every parameter: type, whether required or optional, valid range, default value, and an example.
- Show at least one complete request and response example per endpoint. Use realistic values, not `"foo"` and `123`.
- Document error responses as thoroughly as success responses — errors are what developers hit first.
- Note rate limits, authentication requirements, and idempotency behavior on every mutating endpoint.
- Keep the reference generated from source (OpenAPI, JSDoc, godoc) to eliminate drift.

## Tutorials and Guides

- Structure tutorials with: (1) goal, (2) prerequisites, (3) numbered steps, (4) a working end state.
- Make every step actionable: "Run `npm install`", not "Install the dependencies".
- Include expected output after commands that produce it. Readers need to know what success looks like.
- State assumptions explicitly in prerequisites: OS, version, permissions. Undocumented assumptions are the most common cause of tutorial failure.
- Test tutorials against a clean environment before publishing. What works from your machine may not work for a reader.

## Changelogs

- Follow Keep a Changelog format: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security` sections.
- Write from the user's perspective: "Added `--dry-run` flag to `install`" not "Implemented dry run feature internally."
- Every breaking change gets a migration note: what changed, why, and how to update.
- Link to issues or PRs for traceability.

## Maintenance

- Store docs in the same repo as the code they document. Co-located docs get updated with code; separate docs rot.
- Add a "last verified" date to tutorials and guides. Readers need to know when to distrust stale content.
- Treat broken docs like broken tests — file issues and fix promptly.
- Review docs in every PR that changes an externally visible interface.

## Checklist

- [ ] Audience is named and prose is addressed to that audience.
- [ ] All API parameters documented with type, required/optional, default, and example.
- [ ] Success and error responses both shown with realistic examples.
- [ ] Tutorial tested against a clean environment.
- [ ] Changelog entry follows Keep a Changelog format with breaking change migration notes.
- [ ] Docs live alongside the code they document.
