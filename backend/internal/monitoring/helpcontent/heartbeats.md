---
slug: heartbeats
title: Heartbeats
summary: Checks where an external job pings HealthOps. If the ping is missed, an incident opens.
intent: Use heartbeats to monitor cron jobs, batch workers, scheduled exports, and anything that runs on its own clock instead of being polled.
category: Operate
order: 130
icon: activity
relatedPaths:
relatedTopics: checks,incidents,api-quickstart
---

# Heartbeats

A heartbeat check is the inverse of a normal check. Instead of HealthOps polling a target, **your job pings HealthOps**. If the ping is late, an incident opens.

## When to Use a Heartbeat

- Nightly database backups
- Cron jobs (data exports, billing runs, report generators)
- Periodic workers (queue drainers, sweep jobs)
- Anything where "the job did not run" is the failure mode

## How It Works

1. You create a heartbeat check with an `intervalSeconds` and a `graceSeconds`.
2. HealthOps issues a unique URL: `POST /api/v1/heartbeats/{token}`.
3. Your job calls that URL at the end of each successful run.
4. If no ping arrives within `intervalSeconds + graceSeconds`, an incident opens.
5. The next successful ping resolves the incident.

## Pinging from a Shell Script

```bash
# end of your cron script
curl -fsS -m 10 -o /dev/null \
  https://healthops.example.com/api/v1/heartbeats/abc123def456 || true
```

The `|| true` keeps the cron job from failing if the ping itself fails. Heartbeat misses are detected by HealthOps; ping errors are not the cron job's responsibility.

## Pinging from Code

```python
import requests
requests.post("https://healthops.example.com/api/v1/heartbeats/abc123def456", timeout=10)
```

The ping URL is **unauthenticated by token only** — anyone with the token can ping. Treat the URL as a secret.

## Tuning Grace Period

- A 5-minute cron with a 1-minute grace fires within 6 minutes of a missed run.
- A nightly job that sometimes takes 2 hours should set `graceSeconds: 7200` (or more).
- Too short a grace produces false alerts on every slow night.

## Pinging at the Start vs End of the Job

End is correct. Start-of-job pings mean "I started", not "I succeeded". If the job hangs or crashes, the start-ping already happened and HealthOps will not notice.

## Multiple Stages

For jobs with critical stages, use one heartbeat per stage. "Backup ran" and "Backup verified" are different signals.

## Common Pitfalls

- **"My cron runs at 2 AM but the alert came at 2:15"** — your interval is set too short. The interval is how often you ping, not the cron schedule. Set `intervalSeconds` to slightly more than the cron interval.
- **"It alerts every time my server reboots"** — that is the point. The ping was missed. Either widen grace or accept the alert.
