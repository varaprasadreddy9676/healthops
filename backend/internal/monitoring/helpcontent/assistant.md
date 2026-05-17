---
slug: assistant
title: Ask AI
summary: A conversational assistant for asking operational questions about HealthOps data.
intent: Use Ask AI for guided investigation. It should always explain what data it used.
category: AI
order: 420
icon: message-square
relatedPaths: /assistant
relatedTopics: ai-overview,root-cause
---

# Ask AI

Ask AI is a chat surface over HealthOps' own data. Useful for the questions that do not have a dashboard tile yet.

## What It Can Answer

- Summaries of current incidents and unhealthy checks.
- Recent patterns ("what's been failing most this week?").
- Affected services and possible next diagnostics.
- Explanations of HealthOps concepts (heartbeat checks, log ingestion, status pages).

## What It Cannot Do

- Execute changes on its own.
- Reach external systems beyond HealthOps' own data.
- Replace incident review — it cannot see things HealthOps does not see.

## How To Use It Well

1. Be specific. "What is the most failed check in the last 24 hours and what was its last error?" beats "what's wrong?".
2. Ask for evidence. "Show me the check IDs you used to answer that."
3. Cross-check answers against the underlying pages before acting.

## Conversations and History

Conversations persist so you can refer back. Old conversations can be deleted.

## When AI Is Not Configured

The page is hidden.
