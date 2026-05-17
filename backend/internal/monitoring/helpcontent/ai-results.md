---
slug: ai-results
title: AI Results
summary: Saved AI analyses for incidents and operational signals.
intent: Use this page to audit prior AI outputs and compare them against the underlying incidents.
category: AI
order: 430
icon: history
relatedPaths: /ai
relatedTopics: ai-overview,root-cause
---

# AI Results

Every AI analysis is stored. This page lists them so you can audit what was asked and what was answered.

## Why Audit AI Output

- To confirm the model used real evidence, not invented facts.
- To find prompt regressions when answers get worse.
- To justify decisions made on AI suggestions ("we acted on this analysis because…").

## Review Checklist

1. **Provider, model, timestamp.** Different models give different answers; track which one you trusted.
2. **Confidence.** Low-confidence answers should not have driven action.
3. **Open the linked incident** and re-check the evidence the model cited.
4. **If something is wrong**, flag it — many providers let you submit feedback.

## What This Page Does Not Do

- Edit the AI output. Outputs are immutable.
- Re-run the analysis from this view — use the incident page for that.
- Show you AI failures (those are in the AI queue / provider health view).

## When AI Is Not Configured

The page is hidden.
