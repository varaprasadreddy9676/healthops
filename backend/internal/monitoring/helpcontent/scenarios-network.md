---
slug: scenarios-network
title: Scenarios — Network
summary: Step-by-step recipes for monitoring TCP ports, DNS records, ping, and SSH reachability.
intent: Pick the recipe that matches your network monitoring need and paste the configuration.
category: Scenarios
order: 620
icon: network
relatedPaths:
relatedTopics: checks,servers
---

# Scenarios — Network

## Recipe 1 — Monitor a TCP port is open

**Goal:** Confirm port 6379 (Redis) is reachable on `cache.example.com`.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `tcp`
   - Target: `cache.example.com:6379`
   - Interval: `30` seconds
   - Timeout: `5` seconds
   - Failures to open: `3`
3. Save and **Run now**.

**Equivalent API call:**

```bash
curl -X POST $H/api/v1/checks -H "Authorization: Bearer $T" -H "Content-Type: application/json" -d '{
  "id":"redis-cache","name":"Redis cache","type":"tcp",
  "target":"cache.example.com:6379","intervalSeconds":30,"timeoutSeconds":5,"failuresToOpen":3
}'
```

**Use this for:** any port — Postgres (`5432`), MySQL (`3306`), Kafka (`9092`), SSH (`22`), custom services.

**Caveat:** TCP open ≠ service healthy. A process can bind a port without handling traffic. Pair with an API check or a custom command check that actually exercises the service.

---

## Recipe 2 — Monitor DNS resolves to the right record

**Goal:** Catch the day someone deletes the `A` record for `api.example.com`.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `dns`
   - Hostname: `api.example.com`
   - Record type: `A`
   - Expected value: `203.0.113.42` (or a substring / pattern your records always include)
   - DNS server (optional): `1.1.1.1`
   - Interval: `300` seconds
3. Save.

**Use the DNS server field** to test resolution from a specific resolver, useful when you suspect a caching issue at a particular provider.

**Other record types:** `CNAME` for aliases, `MX` for mail, `TXT` for SPF/DKIM/site-verification.

---

## Recipe 3 — Monitor a host with ping

**Goal:** Confirm `gateway-1.example.com` is reachable.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `ping`
   - Target: `gateway-1.example.com`
   - Packet count: `3`
   - Interval: `60` seconds
   - Acceptable packet loss: `0` (or `33` if you tolerate one of three lost)
3. Save.

**Caveat:** Many cloud and corporate networks block ICMP. A ping failure may mean "ICMP blocked", not "host down". When in doubt, use a TCP check on a known-open port.

---

## Recipe 4 — Monitor that SSH login works

**Goal:** Catch the case where your SSH key was rotated out of `~/.ssh/authorized_keys` without telling HealthOps.

**Steps:**

1. Make sure the target server is registered: **Servers → Add Server** with `hostname`, `sshUser`, `sshKeyPath`.
2. **Checks → Add Check**.
3. Fill:
   - Type: `ssh`
   - Server: select the registered server
   - Interval: `300` seconds
   - Timeout: `15` seconds
4. Save.

**What it does:** opens an SSH session, runs a no-op (`true`), closes. Pass = success. Failure = authentication or connectivity broken.

**Pair with:** a `command` check on the same server for end-to-end coverage. If SSH works but commands fail, you have a shell/PATH issue.

---

## Common Tuning Notes For Network Checks

- **Where you check from matters.** A check passes from the HealthOps host but may fail from a different network segment. Add additional HealthOps deployments if you need multi-vantage coverage.
- **Burst-rate is your enemy.** Network checks at 1-second intervals can flood corporate firewalls. Keep intervals at 30s or more.
- **Maintenance windows** are useful for planned network changes; schedule them before the change so HealthOps does not alert you on every step.
