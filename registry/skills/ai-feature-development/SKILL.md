---
name: ai-feature-development
description: Build AI-powered features with disciplined prompt engineering, evaluation, and observability. Use when integrating LLMs, building agents, or shipping AI capabilities.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.2.0"
---

# AI Feature Development

AI features fail in production for the same reasons all software fails: inadequate specification, no regression harness, and invisible failures. Treat LLM calls like external I/O with latency and failure modes.

## Prompt Engineering

- Write the system prompt as a spec: role, output format, constraints, and examples. Vague prompts produce vague outputs.
- Delimit inputs with XML tags: `<document>`, `<instructions>`, `<examples>`. Prevents injection and makes parsing explicit.
- Prefer structured output (JSON schema, tool/function calling) over parsing free-form text in production code.
- Version prompts alongside code. A prompt change is a code change — review it, test it, deploy it with a version tag.
- Never interpolate raw user input into a prompt without explicit boundary tags and instruction separation.

## Model Selection and Routing

- Use the smallest model that meets quality requirements — latency and cost scale with model size.
- Route simple classification or extraction to smaller, faster models; reserve large models for reasoning-heavy steps.
- Always specify a model version explicitly — never use a `latest` alias in production.
- Build the abstraction so swapping models requires one config change, not a refactor.

## Evaluation

- Every AI feature needs an eval suite before shipping: golden examples, edge cases, and regression cases.
- Measure: accuracy/recall on the task, latency (p50/p95), cost per call, and failure/refusal rate.
- Run evals in CI. A prompt change that passes evals can ship; one that doesn't cannot.
- Use LLM-as-judge only for rubric-graded tasks where human labeling is too expensive — always validate judge agreement against human labels on a sample.

## Reliability

- Wrap all LLM calls with retry (exponential backoff with jitter) and circuit breakers.
- Set explicit timeouts: reading tokens can hang indefinitely without one.
- Define and enforce token budgets (max input + output) per call path.
- Implement graceful degradation: define what the feature does when the LLM is unavailable, over rate limit, or returning unexpected output.

## Observability

- Log every LLM request/response pair with trace ID, model ID, prompt version, latency, token counts, and finish reason.
- Emit metrics: request count, error rate, latency p95, tokens/request, cost/request.
- Sample raw inputs/outputs for quality monitoring — apply PII scrubbing before storing.

## Safety and Security

- Validate and sanitize user inputs before including in prompts.
- Treat model outputs as untrusted: validate structure, length, and content before acting on them.
- Never expose raw model outputs in consumer-facing UIs without a content safety check.
- Apply output validation schemas with strict typing; reject malformed responses rather than silently degrading.

## Checklist

- [ ] Prompt is versioned and reviewed as code.
- [ ] Eval suite covers happy path, edge cases, and known failure modes.
- [ ] Evals run in CI with a pass/fail gate.
- [ ] LLM calls have timeouts, retries, circuit breaker, and fallback behavior.
- [ ] All invocations logged with trace ID and prompt version.
- [ ] Token budgets enforced per call path.
