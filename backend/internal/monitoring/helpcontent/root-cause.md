---
slug: root-cause
title: Root Cause
summary: AI-assisted incident analysis built from HealthOps evidence.
intent: Use this only after AI is configured. Treat it as a second opinion over the evidence, not the source of truth.
category: AI
order: 410
icon: search-code
relatedPaths: /rca
relatedTopics: ai-overview,incidents,ai-results
---

# Root Cause

When an incident opens, HealthOps can enqueue an AI analysis of the evidence and present the result on the incident.

## How It Works

1. Incident opens; HealthOps captures an evidence snapshot.
2. A worker pulls the incident from the AI queue.
3. It builds a prompt from the configured template plus the evidence.
4. It calls the configured AI provider.
5. The response is stored as an `AIAnalysisResult` linked to the incident.
6. The incident page shows the analysis in a separate panel.

## What the Analysis Includes

- A suggested cause (the AI's best guess).
- The evidence the model used.
- A confidence indicator.
- Suggested next checks or diagnostics (sometimes).

## How To Validate It

1. **Confirm the suggested cause matches the raw evidence.** If the model references a fact you cannot see in the snapshot, reject the analysis.
2. **Prefer specific next checks over broad remediation advice.** "Restart everything" is rarely the right answer.
3. **Read the prompt and the response together.** A leading prompt produces a leading answer.

## On-Demand Re-Analysis

You can re-run analysis from the incident page. Useful when:

- New evidence was attached after the original run.
- You changed the prompt template.
- The first analysis was clearly wrong and you want a second attempt.

Repeated re-runs cost API calls. Budget accordingly.

## When AI Is Not Configured

Root Cause is hidden. No fake analysis is shown.

## Common Pitfalls

- **"The analysis hallucinated."** That is why this is a second opinion, not truth. Validate against the snapshot.
- **"It is always 'database is slow'."** Your prompt may be too generic. Tune it to the kinds of incidents you actually see.
- **"It never finishes."** Provider may be timing out. Check provider health in Settings.
