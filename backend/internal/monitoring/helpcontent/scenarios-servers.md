---
slug: scenarios-servers
title: Scenarios — Servers
summary: Step-by-step recipes for monitoring processes, disk, CPU, custom commands, and log file freshness on Linux hosts.
intent: Pick the recipe that matches your host monitoring need and paste the configuration.
category: Scenarios
order: 630
icon: terminal
relatedPaths:
relatedTopics: checks,servers
---

# Scenarios — Servers

Every recipe below requires the target host to be registered in **Servers** with SSH credentials. HealthOps connects via SSH to run the check.

## Recipe 1 — Monitor a systemd service or process is running

**Goal:** Open an incident if the `nginx` process is no longer running on `web-1`.

**Steps:**

1. **Servers** → confirm `web-1` is registered with SSH key.
2. **Checks → Add Check**.
3. Fill:
   - Type: `process`
   - Server: `web-1`
   - Process keyword: `nginx: master`  *(unique substring from `ps -ef` output)*
   - Interval: `60` seconds
   - Failures to open: `2`
4. Save and **Run now**.

**Why "nginx: master" not "nginx"?** The keyword should match exactly one process. `nginx` alone may match `nginx-debug`, `tail -f nginx.log`, etc.

**For systemd:** use `command` instead and run `systemctl is-active <unit>`:

```
type: command
server: web-1
command: systemctl is-active nginx
expectedExitCode: 0
expectedOutputContains: active
```

---

## Recipe 2 — Monitor disk space on a server

**Goal:** Warn at 80% disk usage on `/`, open incident at 90%.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `command`
   - Server: `web-1`
   - Command: `df -P / | awk 'NR==2 {gsub("%","",$5); exit ($5 >= 90 ? 2 : ($5 >= 80 ? 1 : 0))}'`
   - Expected exit code: `0`
   - Interval: `120` seconds
   - Failures to open: `1`
3. Save.

**How this works:** the command exits `0` healthy, `1` warning, `2` critical. HealthOps treats non-zero as failure; the `2` will register as a stronger signal in newer builds.

**Simpler alternative:** repeat for `/data`, `/var/log`, and any other mount you care about. One check per mount.

**For multi-mount visibility:** if a server has a metrics agent reporting disk, you will see disk usage on the **Servers** detail page without a check.

---

## Recipe 3 — Monitor CPU and memory pressure

**Goal:** Alert when load average is sustained above 4 on a 2-core machine.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `command`
   - Server: `web-1`
   - Command: `awk '{print ($1 > 4.0)}' /proc/loadavg | grep -q 1 && exit 1 || exit 0`
   - Expected exit code: `0`
   - Interval: `60` seconds
   - Failures to open: `5` (avoid one-spike alerts)
3. Save.

**For memory:**

```
command: awk '/MemAvailable/ {if ($2 < 524288) exit 1}' /proc/meminfo
```

That fires when free memory drops below 512 MB. Tune to your host.

---

## Recipe 4 — Monitor a custom shell command

**Goal:** Anything you can express as "this command should exit 0 and (optionally) print something".

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `command`
   - Server: target
   - Command: your command, exactly what you would type at a shell
   - Expected exit code: `0`
   - Expected output substring: `<text you require in stdout>`
   - Interval: pick what makes sense
3. Save.

**Examples:**

- `curl -fsS localhost:8500/v1/status/leader` — confirm Consul has a leader.
- `psql -h db -U mon -c 'SELECT 1' postgres` — confirm Postgres login works.
- `redis-cli -h cache ping | grep -q PONG` — confirm Redis answers.
- `kafka-broker-api-versions --bootstrap-server localhost:9092` — confirm broker responds.

**Always run the command by hand first** as the SSH user HealthOps uses. If it fails there, it will fail here too.

---

## Recipe 5 — Monitor a log file is fresh

**Goal:** Detect when an app stops writing to `/var/log/app.log` — usually means the app crashed or got stuck.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `log`
   - Server: target
   - Path: `/var/log/app.log`
   - Max age: `300` seconds (5 minutes)
   - Interval: `60` seconds
3. Save.

**Optional content check:** add `requiredSubstring: "heartbeat"` to also require that the latest line contains a known marker.

**Tail-friendly logger required:** some loggers buffer and only flush on rotation. Confirm your logger flushes regularly, or watch a different file.

---

## Common Tuning Notes For Server Checks

- **One SSH session per check run** — heavy check counts on one host may exhaust SSH `MaxSessions`. Raise it, or batch with one consolidated command check.
- **Use a dedicated low-privilege user** for HealthOps SSH. Sudo only when needed and scope it tightly via `/etc/sudoers.d/`.
- **Stale "last seen"** on the Servers page means SSH did not connect on the last attempt — check key permissions and `sshd` logs on the host.
