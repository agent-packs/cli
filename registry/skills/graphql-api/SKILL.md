---
name: graphql-api
description: Design and implement GraphQL APIs with correct schema design, resolver patterns, N+1 prevention, federation, and security. Use when building or evolving a GraphQL layer.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# GraphQL API

A GraphQL schema is a public contract. Design it for consumers, not for your data model.

## Schema Design

- Name types after domain concepts, not database tables: `Order`, not `OrdersTableRow`.
- Use Connections (Relay cursor pagination) for any list that can grow: `orders(first: Int, after: String): OrderConnection`.
- Prefer non-null (`!`) by default; make a field nullable only when null carries semantic meaning ("order has no coupon"). Nullable fields are a footgun for clients.
- Design mutations as operations, not CRUD verbs: `placeOrder`, `cancelOrder`, not `createOrder`, `deleteOrder`.
- Input types should be specific to the mutation: `PlaceOrderInput`, not a generic `OrderInput` reused across create/update.
- Never expose internal IDs directly. Use opaque global IDs (`base64("Order:123")`) so the type can be changed without breaking clients.

## Resolvers

- Keep resolver logic thin: delegate to service/repository layer, never embed business logic or SQL in a resolver.
- Parent resolvers return a reference (e.g., `{ id }`) and field resolvers hydrate only the fields requested. Avoid loading all fields when only one is needed.
- Use DataLoader for every field that resolves related entities. A field that loads a related type in a loop is an N+1 — it will not be obvious in development traffic.
- DataLoader keys must be stable and comparable. Coerce IDs to strings before using them as cache keys.

## N+1 Prevention

- Profile queries with `graphql-query-complexity` or similar to detect expensive operations before they reach production.
- Set a complexity limit per request (`maxComplexity: 1000`) and a depth limit (`maxDepth: 10`). Enforce at the gateway.
- Use `JOIN` or `IN` queries in DataLoader batch functions — never loop and call one-at-a-time.
- Log the number of SQL queries per GraphQL request during development. More than ~10 queries for a single request signals an N+1.

## Federation and Gateways

- Define `@key` directives on types that are shared across subgraphs. Every federated entity needs a `__resolveReference` resolver.
- Subgraphs own their slice of the schema — never duplicate type definitions across subgraphs. Use `@external` and `@requires` for cross-subgraph fields.
- Test the composed supergraph schema in CI using `rover supergraph compose`. Composition errors are breaking changes.
- Use persisted queries (APQ or trusted documents) in production to prevent arbitrary query execution and reduce payload size.

## Security

- Disable introspection in production (or restrict it to authenticated users). Introspection maps your schema for attackers.
- Validate and sanitize all string inputs at the resolver boundary — GraphQL type safety does not sanitize values.
- Rate limit at the operation level, not just HTTP. A single complex query can be more expensive than 1,000 simple ones.
- Audit resolvers for authorization: every resolver that returns sensitive data must check permissions, even if the parent resolver did.

## Checklist

- [ ] All list fields use Connections (cursor pagination).
- [ ] Every resolver that loads related types uses DataLoader.
- [ ] Complexity and depth limits enforced at the gateway.
- [ ] Introspection disabled or restricted in production.
- [ ] Every mutation has a specific Input type.
- [ ] Persisted queries enabled for production clients.
- [ ] Authorization checked in every sensitive resolver, not just at the root.
