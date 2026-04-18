# ADR 003: Authentication Strategy

## Context
The current backend prototype has no authentication or authorization on write APIs. As we move to a production-grade system with capabilities to create, edit, disable, or delete checks, we must prevent unauthorized mutation.

## Decision
We will implement an authentication model based on a **Static API key or signed service token** from environment variables or configuration. This key/token will be required to access any mutating API endpoints (e.g., editing checks, acknowledging incidents, triggering manual runs). 

## Consequences
**Positive:**
- Easy to implement and understand.
- Sufficient security boundary for a single-tenant internal ops tool.
- Straightforward configuration for CI/CD pipelines and internal CLI/automation scripts.

**Negative:**
- Key rotation requires service restart or re-deployment.
- Granular RBAC (Role-Based Access Control) is not supported.

**Alternatives Considered:**
- **Full OIDC/OAuth2 Integration:** We considered integrating with an identity provider. This was deemed too heavy for the V1 internal single-tenant tool, adding unnecessary complexity. We can revisit this in later phases if broader user groups need to manage the system.
