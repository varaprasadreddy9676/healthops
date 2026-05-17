---
slug: rca-reports
title: RCA Reports
summary: Browse every AI-generated root-cause analysis across all incidents — find patterns, audit quality, export for post-mortems.
intent: Use this when you want the cross-incident view of AI output, not just a single incident's RCA. Best for weekly reliability reviews and AI quality audits.
category: AI
order: 415
icon: file-text
relatedPaths: /rca
relatedTopics: root-cause,ai-results,incidents,ai-overview
---

# RCA Reports

A single incident shows you *its* RCA. This page shows you *all* RCAs at once — the only way to spot patterns that no single incident reveals.

## What You See

A reverse-chronological list of every RCA HealthOps has generated. Each row shows:

| Column | Meaning |
| ------ | ------- |
| Incident ID | Click to open the source incident |
| Check / monitor | Which monitor triggered it |
| Severity | `critical` / `warning` / `info` |
| Opened-at | Wall-clock time the incident opened |
| Provider / model | Which AI generated this RCA |
| Confidence | The model's self-reported confidence (0–1) |
| Recommendation snippet | First line of the suggested fix |

Click a row to expand the full RCA — the same content as the Root Cause panel on the incident page, plus the evidence trail.

## What This Page Is For

| Use case | How to use it |
| -------- | ------------- |
| **Pattern detection** | Filter by check or severity. Three RCAs in 2 weeks all blaming the same downstream? That is your next reliability project. |
| **AI quality audit** | Read 20 RCAs in a row. If half are "investigate the database" with no specifics, your prompt template is too vague — tune it in Settings. |
| **Provider comparison** | Run a week with provider A, a week with B. Filter by provider. Compare recommendation specificity. |
| **Weekly review** | Date-range filter to the past 7 days. Skim every critical RCA. Find the three worst, talk about them in the next reliability meeting. |
| **Post-mortem prep** | Find the RCA for the incident, copy the structured summary into your post-mortem template, save 20 minutes of writing. |

## Filters

| Filter | Useful for |
| ------ | ---------- |
| Severity | Focus on `critical` for the weekly review |
| Provider | Comparing output quality |
| Date range | Narrow to a deploy window or post-mortem period |
| Confidence threshold | Hide low-confidence RCAs the model itself was unsure about |
| Check / monitor | All RCAs for the same monitor |
| Marked incorrect | RCAs an operator flagged as wrong |

## Exporting

| Format | Contents |
| ------ | -------- |
| CSV | One row per RCA: incident, check, severity, provider, confidence, recommendation snippet, date |
| JSON | Full structured fields plus body — pipe into your own analytics |

Use the export when you want to:

- Share a customer post-mortem packet.
- Build a dashboard outside HealthOps.
- Run your own pattern detection (e.g. cluster RCAs by recommendation text).

## Empty States

| What you see | Why | Fix |
| ------------ | --- | --- |
| "No RCAs yet" | No incidents have triggered AI, or AI is disabled | Confirm AI is enabled in Settings; let the system run |
| "AI is disabled" | No provider configured | Settings → AI Providers → add one |
| "All RCAs filtered out" | Filters too narrow | Clear filters; start over |

## Operator Feedback Loop

Each RCA has two operator actions:

- **Mark as helpful** — useful when reviewing later for what is working.
- **Mark as incorrect** — the signal we use to identify prompt or provider failures.

These flags appear in the AI quality dashboard (Settings → AI → Quality) and inform future prompt tuning.

## What a Good RCA Looks Like

- One-sentence root cause.
- Three to five specific contributing signals (with values, not generalities).
- Two or three suggested next actions, ranked by reversibility.
- Confidence between 0.5 and 0.9 (1.0 confidence on a complex incident is suspicious).
- The evidence trail cites real values from the snapshot — not generic advice.

## What a Bad RCA Looks Like

- "Investigate the database." (No specifics.)
- "Likely a network issue." (Unfalsifiable.)
- Confidence 0.95 on an incident with three plausible causes. (Overconfident.)
- Recommends an action that contradicts what the evidence shows. (Hallucinated.)

When you see a bad RCA, mark it incorrect, then either tune the prompt or switch the provider for that incident type.

## Common Pitfalls

| Symptom | Cause | Fix |
| ------- | ----- | --- |
| Same RCA text for every incident | Prompt template ignores incident-specific context | Add `{{ .Evidence }}` and `{{ .CheckType }}` to template |
| Confidence is always 0.9+ | Provider is overconfident on this incident class | Try a different provider or prompt asking for explicit uncertainty |
| RCA cites data that is not in the snapshot | Model hallucinated | Mark incorrect; consider a stricter prompt |
| No RCAs for warning-severity incidents | Default config only runs RCA on critical | Settings → AI → RCA trigger severity |

## Where to Go Next

- **AI Overview** — understand what AI sees and what it cannot.
- **Root Cause** — single-incident RCA view.
- **AI Results** — raw history of every AI call, not just RCAs.
- **Incidents** — the source of every RCA.
