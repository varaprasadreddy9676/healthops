---
slug: ai-overview
title: AI Overview
summary: What HealthOps' AI surfaces actually do, how BYOK works, exactly what gets sent to the provider, and where the safety boundaries are.
intent: Read this before enabling AI. It is the only page that explains what AI sees, what it can do, what it cannot do, and where the security trust boundaries sit.
category: AI
order: 400
icon: sparkles
relatedPaths:
relatedTopics: root-cause,assistant,ai-results,remediation,rca-reports
---

# AI Overview

HealthOps' AI surfaces are **optional, off by default, and BYOK**. With no provider configured, every AI page in the UI is hidden — there is no pretending that AI is running when it is not.

## What AI Can Do

| Surface | What it does | When it runs |
| ------- | ------------ | ------------ |
| **Root Cause** | Reviews incident evidence, suggests likely causes + next actions | Auto on incident open (configurable), on-demand from incident view |
| **Ask AI** | Conversational query over current HealthOps state ("what is unhealthy right now?") | When you type in the assistant box |
| **AI Results** | Stores raw outputs of every AI call — auditable history | Automatic |
| **RCA Reports** | Aggregated view of every RCA across incidents | Reads from AI Results |
| **Remediation suggestions** | Proposes guided action steps for known patterns | On incident open, only when no built-in playbook matches |
| **Log categorize** | Assigns categories to log patterns the rule engine could not classify | On log ingest (if enabled) |

## What AI Cannot Do — The Hard Boundaries

| Action | Allowed? |
| ------ | -------- |
| Read incident evidence (already in HealthOps) | yes |
| Read your source code | no — never sent |
| Read raw application logs not ingested into HealthOps | no |
| Access your cloud account, CI, secrets | no |
| Mutate any HealthOps configuration | no |
| Execute commands on your infrastructure | no — only operator-approved remediation can run, and AI cannot self-approve |
| Resolve incidents | no — operators only |
| See or store API keys, DSNs, credentials | no — DSNs are env-var-only, keys are AES-256-GCM encrypted, never returned by the API |

## BYOK — Bring Your Own Key

HealthOps does not ship an AI provider, a hosted key, or a default model. You configure one or more providers:

| Provider | Models | Best for |
| -------- | ------ | -------- |
| OpenAI | GPT-class | General RCA, strong reasoning |
| Anthropic | Claude-class | Long-context RCA on dense evidence |
| Google Gemini | Gemini-class | Cost-sensitive RCA |
| Ollama | local models (Llama, Mistral, etc.) | Air-gapped / offline deployments |
| Custom | any OpenAI-compatible endpoint | Self-hosted vLLM, LiteLLM proxies, internal gateways |

You bring the API key. HealthOps:

- Stores the key **AES-256-GCM encrypted** at rest in MongoDB.
- Keeps the encryption key in `data/.ai_enc_key` (mode 600, never logged).
- **Masks the key** in every API response and UI surface — there is no "show key" button.
- Supports rotating both the API key and the encryption key (see `cmd/rotate-ai-keys`).

## Exactly What Gets Sent to the Provider

For every AI call, the prompt contains *only* the following:

```
[ system prompt — the template you configure ]
[ incident metadata — id, type, opened_at, severity, monitor name ]
[ incident evidence — check results, MySQL snapshot, server metrics,
                      related log lines IF you ingest them ]
[ user prompt — the question, e.g. "Suggest root cause" ]
```

Not sent:

- Your application source code.
- Any data not already in HealthOps.
- Other users' incidents.
- API keys, DSNs, or any secret.
- Audit log content beyond the incident in scope.

If your check messages or log lines contain PII or secrets, redact at the source — HealthOps will forward whatever evidence the check captured.

## Provider Health, Fallback, and Cost Control

```
   ┌──────────────┐   request    ┌────────────┐    on failure   ┌────────────┐
   │  AI service  │ ───────────▶ │ primary AI │ ──────────────▶ │ fallback 1 │
   │  (worker)    │              │  provider  │                 └─────┬──────┘
   └──────────────┘              └────────────┘                       │ on failure
                                                                      ▼
                                                              ┌────────────┐
                                                              │ fallback 2 │
                                                              └────────────┘
```

- Each provider is health-checked periodically.
- If your primary is down or rate-limited, configured fallback providers are tried in order.
- Failures are recorded — the incident still has all its evidence even if no AI call succeeds.
- Per-provider rate limits and per-day spend caps are configurable.

## Cost Awareness

Every enabled AI surface costs API calls. Sensible defaults:

| Lever | Recommendation |
| ----- | -------------- |
| RCA trigger severity | Only `critical` (not warning) |
| Prompt length | Short — long prompts cost more, rarely improve quality |
| Model choice | Cheap model for log categorization, stronger model for RCA |
| Fallback | Use a cheaper provider as fallback when primary is rate-limited |
| Daily cap | Set in Settings → AI; HealthOps stops calling once hit |

## Safety Defaults Worth Knowing

| Default | Why |
| ------- | --- |
| Remediation needs operator approval | Prevents AI-driven incidents-on-incidents |
| AI output is never treated as truth | Operators validate, evidence is canonical |
| AI surfaces hidden until provider is configured + healthy | No phantom UI |
| Every AI call persisted in MongoDB | Full audit trail |
| Encryption key in `.ai_enc_key` (mode 600) | Standard secret hygiene |
| Provider key never returned by API | Even admins cannot exfiltrate it via the UI |

## Common Pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| AI surfaces are hidden in the UI | No provider configured | Settings → AI Providers → add one |
| RCA never appears on incidents | RCA trigger severity excludes this incident's severity | Settings → AI → RCA trigger |
| Provider unhealthy in dashboard | Key invalid, rate-limited, or network blocked | Test the key out-of-band; check egress from the HealthOps host |
| Costs higher than expected | RCA running on warning-severity, or long prompt template | Restrict to `critical`, shorten template |
| RCA hallucinates values not in evidence | Wrong/weak model or prompt asking for too much | Switch model, ask for explicit "based on evidence" phrasing |
| Cannot rotate key | Missing perms on `.ai_enc_key` | See `docs/ai-key-rotation.md` |

## Where to Go Next

- **Root Cause** — single-incident RCA view.
- **RCA Reports** — cross-incident view, exports, quality audit.
- **Assistant** — conversational queries over HealthOps state.
- **AI Results** — raw call-by-call audit log.
- **Remediation** — pair AI suggestions with operator-approved actions.
