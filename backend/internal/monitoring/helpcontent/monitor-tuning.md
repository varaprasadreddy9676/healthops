---
slug: monitor-tuning
title: Monitor Tuning
summary: Recommendations for reducing alert noise and improving monitor coverage.
intent: Use this page to make checks more useful — fewer false positives, clearer thresholds, better coverage of real risks.
category: Understand
order: 320
icon: sliders
relatedPaths:
relatedTopics: checks,alert-rules,analytics
---

# Monitor Tuning

The goal is not "more monitors". The goal is **alerts you act on**. Monitor Tuning analyzes your monitor history and suggests changes.

## How Recommendations Are Produced

HealthOps inspects:

- Incident frequency per check (too many → noisy).
- Mean time-to-resolve per check (very short → false alarm).
- Failure-to-recovery cycles per check (flapping).
- Checks with **zero** incidents and stable runs for months (over-monitored).
- Coverage gaps inferred from server inventory and known services.

When AI is enabled, recommendations may include AI-generated rationale text. The base recommendations are always grounded in monitor data — AI does not invent.

## Good Changes

- **Raise `failuresToOpen`** for noisy but low-risk checks.
- **Lower thresholds** when a check is missing real incidents (under-alerting).
- **Split broad checks** into precise service-level checks ("API up" → "Auth", "Checkout", "Search").
- **Convert pass/fail to alert rules** for graded metrics.
- **Schedule maintenance windows** for predictable noise.
- **Delete** checks that have not produced a useful incident in 90 days.

## When To Ignore a Recommendation

- The check is for a customer-visible flow you must over-monitor.
- The noise is real and the action is to fix the system, not the check.
- The recommendation conflicts with a compliance or SLA requirement.

## The Two-Question Test

For every monitor, ask:

1. **If this fires, what do I do?** If you cannot answer in one sentence, the monitor needs a runbook or should be deleted.
2. **If this never fires, will I notice?** If yes, you have it covered another way and may be over-monitoring.
