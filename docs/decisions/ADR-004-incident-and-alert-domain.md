# ADR 004: Incident and Alert Domain

## Context
Currently, the system only maintains raw state and check run results. There is no concept of ongoing incidents, nor is there a mechanism to notify operators when health changes occur. 

## Decision
We will introduce formal **Incident and Alert Deliveries** as top-level domain concepts.
- **Incident Lifecycle:** An incident is automatically opened when an alert-worthy failure condition is met. Repeated failures will update the existing incident to minimize noise. An incident can automatically resolve upon health recovery (depending on policy), or be explicitly acknowledged and resolved by an operator with notes.
- **Alert Delivery:** Alert rules will evaluate status changes and trigger configurable delivery channels (e.g., Email, Webhook). Cooldowns and deduplications will be embedded in the rule engine to prevent alert fatigue.

## Consequences
**Positive:**
- Operators get a clear, distinct view of active failures vs. transient blips.
- Prevents notification spam by rolling up repeated failures into a single incident entity.
- Enhances accountability and MTTD/MTTR tracking via acknowledgment workflows and audit trails.

**Negative:**
- Increases complexity of the backend data model and state transitions.

**Alternatives Considered:**
- **Stateless Webhooks Only:** We considered just firing webhooks on every state change and relying on third-party systems (like PagerDuty) to handle incidents. We chose to build an incident model internally so that our UI provides a complete day-to-day operational dashboard without requiring additional SAAS subscriptions.
