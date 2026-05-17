---
slug: scenarios-operations
title: Scenarios — Operations and AI
summary: Step-by-step recipes for maintenance windows, alert rules, auto root-cause, and log categorization.
intent: Use these to make HealthOps quieter, smarter, and more useful as the volume grows.
category: Scenarios
order: 690
icon: workflow
relatedPaths:
relatedTopics: maintenance-windows,alert-rules,ai-overview
---

# Scenarios — Operations and AI

## Recipe 1 — Schedule a maintenance window for a deploy

**Goal:** Silence all production checks between 10:00 and 10:30 UTC while you deploy.

**Steps:**

1. **Maintenance Windows → Add Window**.
2. Fill:
   - Name: `Prod deploy 2026-05-17`
   - Starts at: `2026-05-17T10:00:00Z`
   - Ends at: `2026-05-17T10:30:00Z`
   - Scope: tag `prod` (or list specific check IDs)
   - Suppress notifications: `true`
3. Save.
4. Deploy.
5. The window auto-closes at 10:30. If the deploy ran long, edit the window's end time before it closes.

**Recurring deploys:** add `recurrence: weekly` with the same start/end clock time for routine windows.

**Verify the window is active:** in the window list, the active row shows a "live now" indicator. Checks inside still run; incidents do not open.

---

## Recipe 2 — Create an alert rule for a graded metric

**Goal:** Open a warning incident when API p95 latency is above 800 ms for 10 minutes.

**Steps:**

1. **Alert Rules → Add Rule**.
2. Fill:
   - Name: `Checkout API p95 too slow`
   - Target type: `check`
   - Target id: `api-checkout`
   - Metric: `latencyP95`
   - Operator: `>`
   - Threshold: `800`
   - Window: `600` seconds
   - Cooldown: `900` seconds
   - Severity: `warning`
   - Channels: `slack-platform`
3. Save.
4. Verify: artificially slow the check target (Wireshark, tc, whatever) so latency climbs above 800 for 10 minutes. The rule fires.

**Note:** the threshold uses the metric values recorded by the check. If your check is not recording `latencyP95`, the rule never fires. Confirm on the check's history page first.

---

## Recipe 3 — Auto root-cause every critical incident

**Goal:** When a critical incident opens, AI analysis appears within ~30 seconds.

**Steps:**

1. **Settings → AI Configuration → Add Provider** (e.g., Anthropic with your API key).
2. Run **Provider health check**. Confirm OK.
3. **Settings → AI Configuration → Auto RCA**:
   - Trigger: incidents with severity `critical`
   - Provider: your provider
   - Model: a strong reasoning model (cost-aware choice)
   - Prompt template: the default is fine to start; refine after you read a few outputs
4. Save.
5. Generate a critical incident (or wait for one). Open the incident. Within ~30s the **Root Cause** panel should populate.

**Cost control:**

- Limit to `critical` only — most incidents are not worth an AI call.
- Use a smaller/cheaper model for `warning` or none at all.
- AI calls are rate-limited internally; bursts are queued, not amplified.

---

## Recipe 4 — Auto-categorize unclassified log patterns

**Goal:** Get a category on the log patterns the rule-based classifier marks "Unclassified".

**Steps:**

1. **Settings → AI Configuration → Provider** must be configured (Recipe 3 step 1).
2. **Log Events → filter `Unclassified`**. You will see the patterns the rule engine could not match.
3. Click **AI Categorize** at the top of the list.
4. AI assigns categories. Review them — accept or override per pattern.
5. (Optional) For categories you accept, click **Save as Rule** to make the same classification deterministic in the future without burning an AI call.

**Why not run AI on every pattern?** It costs and adds latency. Deterministic rules win — AI is the fallback for the long tail.

---

## Common Tuning Notes For Operations

- **Maintenance windows are not silencers.** Use them only for legitimately planned work. Silencing real incidents to make a graph look better is a path to invisible outages.
- **Alert rules compound — be sparing.** One good rule per metric per service. Ten overlapping rules on the same metric is noise.
- **AI is a multiplier on configuration quality.** Bad prompts give bad answers. Read AI Results regularly and iterate.
- **Audit log is your friend.** When something changes and people disagree about who or why, the audit log is the answer.
