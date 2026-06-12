---
name: accessibility-audit
description: Review and remediate web accessibility issues against WCAG 2.2 AA. Use when building UI components, reviewing HTML, or preparing for an accessibility audit.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.2.0"
---

# Accessibility Audit

Accessibility is a correctness concern, not a polish pass. WCAG 2.2 AA is the minimum bar for public-facing products.

## Semantic HTML

- Use native HTML elements before ARIA: `<button>` over `<div role="button">`, `<nav>` over `<div role="navigation">`.
- Every form input must have a corresponding `<label for="id">` — not just a placeholder.
- Heading hierarchy must be logical (h1 → h2 → h3); never skip levels for visual styling.
- Use `<table>` with `<th scope="col|row">` for tabular data; never use tables for layout.
- Use `<ul>`, `<ol>`, and `<dl>` for list content; don't fake lists with repeated divs.

## Interactive Elements

- All interactive elements must be keyboard-focusable (natural tab order) and operable with Enter and Space.
- Focus must be visible at all times: never `outline: none` without a visible custom replacement that meets 3:1 contrast.
- Modals and dialogs must trap focus while open; restore focus to the trigger element on close.
- Custom widgets (dropdowns, carousels, date pickers, comboboxes) must implement the [ARIA Authoring Practices Guide](https://www.w3.org/WAI/ARIA/apg/) pattern.

## Images and Media

- Every `<img>` must have an `alt` attribute: descriptive text for informational images, empty string (`alt=""`) for decorative ones.
- Videos must have captions (not just auto-generated); audio content must have transcripts.
- SVG icons used as buttons or links need `aria-label` or `<title>` when there is no visible text.

## Color and Contrast

- Normal text (< 18px / < 14px bold): contrast ratio ≥ 4.5:1 against background.
- Large text (≥ 18px / ≥ 14px bold): contrast ratio ≥ 3:1.
- UI component boundaries and focus indicators: ≥ 3:1 against adjacent colors.
- Color alone must never be the sole indicator of information — pair with label, icon, or pattern.

## ARIA Usage

- `aria-label` or `aria-labelledby` is required on any interactive element without visible text.
- Use `aria-live="polite"` for asynchronous updates; `aria-live="assertive"` only for urgent, time-sensitive messages.
- Never override a native implicit ARIA role with the same value — redundant roles add noise.
- `role="presentation"` or `role="none"` removes an element from the accessibility tree — use intentionally.

## Testing

- Run automated scanner (axe-core, Lighthouse, IBM Equal Access Checker) — zero critical/serious violations.
- Navigate the entire flow keyboard-only: every action must be reachable and operable without a mouse.
- Test with a screen reader (VoiceOver on macOS/iOS, NVDA on Windows) for critical user flows.
- Zoom to 200% — no content is clipped, overlapping, or requires horizontal scrolling.
- Test in both light and dark modes for contrast compliance.

## Checklist

- [ ] Automated scan shows zero critical violations.
- [ ] All interactive elements reachable and operable by keyboard.
- [ ] Screen reader test covers at least the happy path.
- [ ] No color-only information conveyance.
- [ ] Zoom to 200% — no loss of content.
- [ ] Form inputs all have associated `<label>` elements.
