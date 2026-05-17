---
slug: ai-overview
title: AI Overview
summary: What HealthOps' AI surfaces do, how BYOK works, what gets sent, and the safety boundaries.
intent: Read this before enabling AI. It explains exactly what AI sees, what it can do, and where it cannot reach.
category: AI
order: 400
icon: sparkles
relatedPaths:
relatedTopics: root-cause,assistant,ai-results,remediation
---

# AI Overview

HealthOps' AI surfaces are optional. They are off by default. With no provider configured, every AI page in the UI is hidden — there is no pretending that AI is running.

## What AI Can Do

| Surface | What it does |
| ------- | ------------ |
| **Root Cause** | Reviews the evidence attached to an incident and suggests likely causes. |
| **Ask AI** | Conversational queries over HealthOps state ("what is unhealthy right now?"). |
| **AI Results** | Stores past AI outputs so you can audit them. |
| **Remediation** | Prepares guided action steps for known incident types. **Never** auto-executes without approval. |
| **AI Categorize** (Log Events) | Assigns categories to log patterns the rule engine could not classify. |

## What AI Cannot Do

- Run commands without explicit approval.
- Modify configuration silently.
- Access data that is not in HealthOps (it does not see your code, your raw logs unless ingested, or your cloud account).
- Override the evidence — operators decide.

## BYOK ("Bring Your Own Key")

HealthOps does not ship an AI provider or a hosted key. You configure one or more providers:

- **OpenAI** (GPT-class)
- **Anthropic** (Claude-class)
- **Google Gemini**
- **Ollama** (local)
- **Custom** (any OpenAI-compatible endpoint)

You bring the API key. HealthOps stores it AES-256-GCM-encrypted at rest in `data/ai_config.json`. The encryption key lives in `data/.ai_enc_key`. API responses always mask keys.

## What Gets Sent to the Provider

A constrained context per request:

- Incident metadata (id, type, opened-at, severity).
- The incident's evidence (check results, MySQL snapshot, server metrics, related signals).
- A configurable prompt template.

Nothing else is sent. No raw application source. No credentials. No user PII unless your check messages happen to contain it (in which case, redact at the source).

## Provider Health and Fallback

HealthOps health-checks each provider. If your primary is down or rate-limited, configured fallback providers are tried in order. Failures are recorded; the incident still has all its evidence.

## Cost Awareness

Each enabled AI surface costs API calls. Recommendations:

- Enable Root Cause only for critical-severity incidents.
- Set the prompt to be brief — long prompts cost more and rarely improve quality.
- Use a small/cheap model for log categorization, a stronger model for RCA.

## Safety Defaults

- Remediation actions require explicit operator approval.
- AI output is **never** treated as audit-quality truth — operators must validate.
- AI surfaces stay hidden until a provider is configured and healthy.
- Logs of AI calls are kept so you can audit what was asked and answered.
