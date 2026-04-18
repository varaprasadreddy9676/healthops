# ADR 001: V1 Scope

## Context
The Medics Health Check system is being transitioned from a legacy Node.js implementation to a production-grade Go backend. We need to clearly define the product scope for V1 to prevent scope creep and ensure timely delivery of a reliable system.

## Decision
We have decided to target a **single-tenant scope** for V1 operations. The product will be an internal operations tool rather than a multi-tenant SaaS offering.

## Consequences
**Positive:**
- Simplifies architecture by avoiding complex data isolation and tenant management logic.
- Faster go-to-market for our internal teams.
- Reduces security surface area.

**Negative:**
- If multi-tenancy is needed in the future, it will require a significant architectural refactor.

**Alternatives Considered:**
- **Multi-tenant SaaS Support:** We considered building multi-tenancy from the start to support multiple isolated clients. This was rejected due to the added complexity and because our immediate need is purely for single-tenant internal operations.
