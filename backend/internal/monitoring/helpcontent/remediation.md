---
slug: remediation
title: Remediation
summary: Suggested or assisted operational actions for known incident types.
intent: Use remediation to standardize response steps. Review every action before running anything against production.
category: AI
order: 440
icon: zap
relatedPaths: /automation
relatedTopics: ai-overview,incidents
---

# Remediation

Remediation turns "I have an incident" into "here are the standard steps to investigate or fix it". Optionally, those steps can be executed with operator approval.

## Assisted, Not Autonomous

Remediation is assisted automation, not silent automation. The flow is:

1. Incident opens with evidence.
2. HealthOps proposes one or more remediation actions for incidents that match a known playbook.
3. **An operator reviews the target, the command, the reason.**
4. The operator approves. Only then does HealthOps run the action.
5. The result is recorded and attached to the incident.

There is no path by which HealthOps silently mutates production.

## What Actions Can Look Like

- A diagnostic command (read-only, e.g., `mysql -e "SHOW PROCESSLIST"`).
- A restart of a known stuck process.
- A scale-up of a worker pool (when wired to your cloud).
- A runbook step (open ticket, page on-call, take screenshot).

## Safety Model

- **Read-only diagnostics first** are encouraged before any mutation.
- Every action displays the target host, command, and reason before execution.
- Every executed action is recorded — what ran, who approved it, what changed in the incident state.
- Destructive actions (`rm`, force-kill, drop, force-push) are blocked by default.

## Where Playbooks Come From

- Built-in playbooks for common patterns (database connection saturation, OOM kills, certificate expiry warnings).
- Custom playbooks you author for your environment.
- AI-suggested actions when AI is enabled — same approval flow applies.

## Common Pitfalls

- **"It approved my action but nothing happened."** The action ran but produced no visible side effect. Read the recorded output before re-running.
- **"It refuses to run."** A safety check blocked the command. Read the explanation and adjust the playbook.
- **"I want full automation."** Out of scope. Approval is intentional. Wrap HealthOps in your own runner if you must.
