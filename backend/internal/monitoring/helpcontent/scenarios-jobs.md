---
slug: scenarios-jobs
title: Scenarios — Scheduled Jobs
summary: Step-by-step recipes for monitoring cron, backups, Kubernetes CronJobs, and serverless functions.
intent: Use these recipes when "the job did not run" is the failure you care about.
category: Scenarios
order: 650
icon: clock
relatedPaths:
relatedTopics: heartbeats,checks
---

# Scenarios — Scheduled Jobs

Scheduled jobs are different from services: nobody polls them, so HealthOps cannot detect a failure from outside. **The job pings HealthOps.** If the ping is late, an incident opens.

## Recipe 1 — Monitor a cron job ran on time

**Goal:** Get paged if your hourly cleanup cron did not run.

**Steps:**

1. **Heartbeats → Add Heartbeat**.
2. Fill:
   - Name: `Hourly cleanup`
   - Interval: `3600` seconds
   - Grace period: `600` seconds (10 minutes — enough for late starts)
3. Save. HealthOps issues a token URL like `https://healthops.example.com/api/v1/heartbeats/abc123`.
4. On your cron server, append the ping to the end of the cron command:

   ```
   0 * * * *   /opt/scripts/cleanup.sh && curl -fsS -m 10 -o /dev/null https://healthops.example.com/api/v1/heartbeats/abc123 || true
   ```

5. Wait one hour. The heartbeat should turn green.

**Why `&&` and `|| true`:**

- `&&` — only ping if cleanup succeeded.
- `|| true` — do not fail the cron if HealthOps is unreachable for a moment.

**Trade-off:** if your script exits non-zero, you will get no ping. That is usually what you want (job failure = alert). If you instead want "job ran, even if it failed", use `;` instead of `&&` and rely on a different signal for success/failure.

---

## Recipe 2 — Monitor a nightly backup completed

**Goal:** Page if last night's database backup did not finish.

**Steps:**

1. **Heartbeats → Add Heartbeat**.
2. Fill:
   - Name: `Prod DB nightly backup`
   - Interval: `86400` seconds (24 hours)
   - Grace period: `7200` seconds (2 hours — backup duration varies)
3. Save and copy the token URL.
4. End of the backup script:

   ```bash
   #!/usr/bin/env bash
   set -euo pipefail

   pg_dump prod | gzip > /backups/prod-$(date +%F).sql.gz
   aws s3 cp /backups/prod-$(date +%F).sql.gz s3://backups-prod/

   # Only reached if both succeeded.
   curl -fsS -m 30 -o /dev/null https://healthops.example.com/api/v1/heartbeats/<token>
   ```

5. On a successful nightly run, the heartbeat stays green. A missed or failed backup fires an incident within 24h+2h.

**Want to monitor "did it run AND was it under 5GB"?** Send to a `command` check pointing at the backup directory; do not overload the heartbeat with content checks.

---

## Recipe 3 — Monitor a Kubernetes CronJob

**Goal:** Alert if a CronJob in Kubernetes did not complete its scheduled run.

**Steps:**

1. Create a heartbeat as above with the right interval.
2. In the CronJob spec, add a final container or a final command in your job container:

   ```yaml
   apiVersion: batch/v1
   kind: CronJob
   metadata:
     name: nightly-report
   spec:
     schedule: "0 2 * * *"
     jobTemplate:
       spec:
         template:
           spec:
             restartPolicy: Never
             containers:
               - name: report
                 image: report:1.2.3
                 command: ["sh","-c"]
                 args:
                   - |
                     /app/run.sh && \
                     curl -fsS -m 10 -o /dev/null \
                       https://healthops.example.com/api/v1/heartbeats/<token>
   ```

3. Apply, wait one cycle.

**Bonus:** add an alert on the Kubernetes side too (cron-job-not-completed events). Defense in depth.

---

## Recipe 4 — Monitor a serverless function invocation

**Goal:** Alert if a scheduled Lambda / Cloud Function stops running.

**Steps:**

1. Create a heartbeat with the function's schedule as the interval.
2. At the end of the function (in the success path), call the ping URL:

   ```python
   import urllib.request

   def handler(event, context):
       do_work()
       try:
           urllib.request.urlopen(
               "https://healthops.example.com/api/v1/heartbeats/<token>",
               data=b"", timeout=5
           )
       except Exception:
           pass  # do not fail the function on ping failure
       return {"status": "ok"}
   ```

3. Deploy. If your scheduler stops invoking the function, the heartbeat fires.

**Notes:**

- Put the ping last. A ping at the start of the function tells you "I was invoked", not "I succeeded".
- For multi-stage functions (chain of Lambdas), use one heartbeat per stage.

---

## Common Tuning Notes For Heartbeats

- **Grace = expected variance.** A nightly backup whose duration varies between 30 min and 2 hours needs ≥ 2h grace.
- **Token is the secret.** Anyone with the token can ping. Do not put it in public repos or client-side code.
- **Re-issue tokens after a rotation incident.** From the heartbeat detail page.
- **"Successful run, missing ping" is the most common failure mode.** Test the ping command independently before trusting the heartbeat.
