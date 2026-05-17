---
slug: rca-reports
title: RCA Reports
summary: Browse historical AI-generated root-cause analyses across incidents, separate from a single incident view.
intent: Use this page to scan all RCAs at once, find patterns across incidents, and audit AI output quality.
category: AI
order: 415
icon: file-text
relatedPaths: /rca
relatedTopics: root-cause,ai-results,incidents
---

# RCA Reports

The Incident view shows the RCA for one incident. The RCA Reports page is the cross-incident view — every RCA HealthOps has generated, sorted, filtered, and exportable.

## What You See

- A list of RCAs across all incidents (latest first).
- For each: incident ID, check, severity, opened-at, the AI provider/model used, confidence score, and a short snippet of the recommendation.
- Click a row to expand the full RCA — same content as the **Root Cause** panel on the incident page.

## Filters

- **Severity** — only critical, only warning, etc.
- **Provider** — useful when comparing output quality between providers.
- **Date range** — narrow to a deploy window or a postmortem period.
- **Confidence threshold** — hide RCAs below a confidence the model self-reported.

## What This Page Is For

- **Cross-incident pattern detection** — three RCAs in two weeks all blame the same downstream service? That is your next reliability project.
- **Quality audit** — read 20 RCAs in a row. If half are "investigate the database", your prompt template needs tuning.
- **Provider comparison** — run a week with provider A, a week with B, compare snippets side by side.

## Exporting

Click **Export → CSV** or **Export → JSON**. The export includes the structured fields plus the full body. Useful for offline review, sharing with a customer in a post-mortem, or piping into your own analytics.

## Empty State

- **"No RCAs yet"** — either no critical incidents have happened, or AI is disabled. Check **Settings → AI Providers**.
- **"AI is disabled"** — turn it on in **Settings → AI Providers** and pick a default provider.

## Tips

- Open an RCA, copy the structured summary into your incident post-mortem template — saves 20 minutes of writing.
- Use the date filter to find "what happened the last time we had a deploy on a Tuesday".
- If an RCA is wrong, click **Mark as incorrect** — that signal feeds prompt-tuning later.
