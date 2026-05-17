---
slug: servers
title: Servers
summary: Inventory and live operating signals for hosts that HealthOps monitors.
intent: Use this page to confirm which machines HealthOps sees and whether their CPU, memory, disk, processes, and network look normal.
category: Operate
order: 110
icon: server
relatedPaths: /servers
relatedTopics: checks,mysql,troubleshooting
---

# Servers

Servers are the hosts HealthOps knows about. A server can be a physical box, a VM, a container, or a logical group (a "service cluster").

## How Servers Get Into HealthOps

- **From server records.** You create them via the API or the Servers page (`POST /api/v1/servers`). Each record can include an SSH key reference so SSH-based checks can target it.
- **From check configuration.** Checks reference a `serverId`; that is enough for the server to appear in views even without a standalone record.

## What You See

- **Inventory** — every known server with its last-seen time.
- **Live signals** — CPU, memory, disk, and load if a metrics agent reports them.
- **Processes** — list of monitored processes and their state.
- **Attached checks** — every monitor that targets this server.

## What To Trust First When Something Is Wrong

1. **Last seen.** If it is old, HealthOps does not have fresh data and your live signals are stale.
2. **CPU and memory** before chasing application errors — saturation explains a lot.
3. **Processes and their PIDs.** If the process you expect is missing, the check failure is real, not a measurement bug.
4. **Then** look at API/MySQL/log signals.

## Adding a Server

`POST /api/v1/servers` with:

```json
{
  "id": "prod-web-01",
  "hostname": "10.0.1.42",
  "sshUser": "deploy",
  "sshKeyPath": "/keys/deploy_rsa",
  "tags": ["prod", "web"]
}
```

SSH keys are read from disk on the HealthOps host. They are never stored in the API or logged.

## Demo Mode

The demo stack ships with two `linux-server` containers (`linux-server-1`, `linux-server-2`) that boot sshd, cron, nginx, and a small HTTP server. SSH/process/command/log checks target them. They are real Linux containers — failures shown there are real failures, not simulated UI state.

## Common Pitfalls

- **"Server is shown unhealthy but I can SSH to it"** — HealthOps' SSH key may not be authorized on that host. Verify with the same user.
- **"CPU shows 0%"** — no agent is reporting metrics. The server still works for SSH/command checks; it just has no live resource graph.
