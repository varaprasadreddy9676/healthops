---
slug: remediation
title: Remediation
summary: Suggested or assisted operational actions for known incident types — approved by a human, executed by HealthOps, recorded as evidence.
intent: Use Remediation to standardize and accelerate incident response. Every action is reviewed before it runs against your infrastructure — there is no silent automation.
category: AI
order: 440
icon: zap
relatedPaths: /automation
relatedTopics: ai-overview,incidents,rca-reports
---

# Remediation

Remediation closes the loop between "the system noticed something is wrong" and "the system also knows the standard fix". Where most monitoring stops at *notification*, HealthOps offers a curated catalogue of fixes — but always behind explicit approval.

> The hard rule: **HealthOps never mutates your infrastructure without an operator clicking "Run".** No exceptions, no opt-out.

## The Flow

```
   ┌───────────────┐   incident opens with full evidence
   │  incident     │ ─────────────────────────────────────┐
   │  + snapshot   │                                       │
   └───────────────┘                                       ▼
                                              ┌──────────────────────┐
                                              │  playbook match?     │
                                              │  - built-in pattern  │
                                              │  - custom playbook   │
                                              │  - AI suggestion     │
                                              └──────┬───────────────┘
                                                     │ yes
                                                     ▼
                                              ┌──────────────────────┐
                                              │  proposed actions    │
                                              │  shown on incident:  │
                                              │  - target host       │
                                              │  - exact command     │
                                              │  - reason            │
                                              │  - blast radius      │
                                              └──────┬───────────────┘
                                                     │
                                          operator   │   click Run
                                          reviews ──▶│
                                                     ▼
                                              ┌──────────────────────┐
                                              │  execute via SSH /   │
                                              │  API / webhook       │
                                              │  + capture output    │
                                              └──────┬───────────────┘
                                                     │
                                                     ▼
                                              ┌──────────────────────┐
                                              │  attach result to    │
                                              │  incident timeline   │
                                              └──────────────────────┘
```

## What Actions Look Like

| Type | Example | Mutates? | Default availability |
| ---- | ------- | -------- | -------------------- |
| Diagnostic | `mysql -e "SHOW PROCESSLIST"` | no | always on |
| Restart | `systemctl restart app-worker` | yes | requires approval |
| Scale | "increase worker pool by 2" (cloud webhook) | yes | requires approval + extra approval |
| Drain | "set node to NoSchedule" (k8s) | yes | requires approval |
| Runbook step | "open ticket in PagerDuty" | no, integration only | always on |
| Notify | "post to #incidents" | no | always on |

Prefer **diagnostics first**. They are read-only, cost nothing to run, and usually tell you whether the mutation is even appropriate.

## Safety Model — In Order of Strictness

1. **Approval required.** Every action shows target, command, and reason. Nothing runs until you click Run.
2. **Destructive commands are blocked by default.** `rm -rf`, `DROP`, force-kill, force-push, etc. are denylisted. To run one, you must edit the playbook explicitly and re-approve.
3. **Two-person rule (optional).** High-impact actions can be configured to require a second operator's approval.
4. **Dry-run mode.** Most playbooks support `--dry-run` to show what *would* happen without doing it.
5. **Full audit trail.** Every action records: who proposed it, who approved it, who ran it, target, command, exit code, stdout, stderr, duration.
6. **Rate limits.** A playbook cannot fire on the same incident more than N times in M minutes.

## Where Playbooks Come From

| Source | What you get | When to use |
| ------ | ------------ | ----------- |
| **Built-in** | Common patterns (DB connection saturation, OOM kills, cert expiry warnings, stuck queue) | Out of the box, no setup |
| **Custom** | YAML you author for your environment | When your incidents have a repeatable fix unique to you |
| **AI-suggested** | When AI is enabled, suggested actions appear on the incident | When the incident is new and no playbook matches |

Custom playbook shape:

```yaml
id: restart-stuck-worker
name: Restart stuck worker
matches:
  - check: api-worker
    severity: critical
  - log_pattern: "worker thread blocked > 60s"
target:
  type: ssh
  serverId: worker-1
action:
  command: "systemctl restart app-worker"
  timeoutSeconds: 30
safety:
  approvalRequired: true
  destructive: false
  maxPerHour: 3
post:
  - waitSeconds: 30
  - reRunCheck: api-worker
```

## What "Good Use" Looks Like

- Three playbooks for your three most common incidents. Not 30.
- Diagnostic playbooks for everything; mutation playbooks only for proven, repeatable fixes.
- Each mutation playbook has been used at least 5 times manually before being added.
- Every action has a `post.reRunCheck` so you can see immediately whether it fixed the problem.

## What "Bad Use" Looks Like

- A playbook that restarts the database when latency is high. (Mask the symptom, lose the evidence.)
- A playbook that disables a check when it fires. (Now you do not know the system is broken.)
- A playbook that runs without telling anyone. (HealthOps does not allow this, but custom integrations might.)
- A `reset_admin_password` playbook with `approvalRequired: false`. (Audit will find this.)

## Common Pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| "It approved but nothing happened" | Action ran, produced no visible side effect | Read the recorded stdout/stderr |
| "It refuses to run" | Safety check blocked the command | Read the explanation; edit playbook only if truly safe |
| "Incident reopens immediately after remediation" | The fix was a symptom mask, not a fix | Stop running that action; root-cause first |
| "I want full automation" | Out of scope by design | Wrap HealthOps in your own runner if you must |
| "Custom playbook does not match my incidents" | `matches:` block too specific | Look at the recent incidents and broaden the match criteria |

## Where to Go Next

- **Incidents** — see the timeline that triggers remediation.
- **AI Overview** — understand how AI-suggested actions are generated.
- **RCA Reports** — pair remediation with post-incident analysis.
