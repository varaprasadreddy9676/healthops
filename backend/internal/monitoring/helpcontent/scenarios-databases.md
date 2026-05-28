---
slug: scenarios-databases
title: Scenarios — Databases
summary: Step-by-step recipes for monitoring MySQL connections, slow queries, replication, deadlocks, and custom queries.
intent: Pick the recipe that matches your database monitoring need and paste the configuration.
category: Scenarios
order: 640
icon: database
relatedPaths:
relatedTopics: mysql,alert-rules
---

# Scenarios — Databases

Every MySQL recipe needs a DSN in an environment variable on the HealthOps host. Set it before creating the check, then reference it by env name. DSNs are never stored or logged.

## Setup (do once per database)

1. Create a read-only monitoring user:

   ```sql
   CREATE USER 'healthops'@'%' IDENTIFIED BY 'strong-random-password';
   GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'healthops'@'%';
   GRANT SELECT ON performance_schema.* TO 'healthops'@'%';
   FLUSH PRIVILEGES;
   ```

2. Set the DSN on the HealthOps process (systemd unit, container env, whatever you use):

   ```
   MYSQL_PROD_DSN=healthops:strong-random-password@tcp(db.example.com:3306)/
   ```

3. Restart HealthOps so it sees the new env var.

---

## Recipe 1 — Monitor MySQL connection saturation

**Goal:** Open an incident when active connections cross 90% of `max_connections` for 5 minutes.

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `mysql`
   - Name: `Prod MySQL`
   - DSN env: `MYSQL_PROD_DSN`
   - Interval: `60` seconds
3. Save.
4. Go to **Alert Rules**. The default "Connection utilization" rule is already enabled and watches every MySQL check. Tune its threshold to `90` and window to `300` seconds.

**Verify:** open `mysql>` and run `SET GLOBAL max_connections = 10;`. Open 9 sessions. The metric jumps; the rule fires in ~5 minutes.

---

## Recipe 2 — Monitor MySQL slow queries

**Goal:** Open an incident if more than 10 slow queries per minute occur on the database.

**Steps:**

1. Confirm `slow_query_log = ON` on the database (HealthOps watches the counter, not the log file).
2. Set `long_query_time` to a sensible value (e.g., `2` seconds).
3. With the `mysql` check in place from setup, go to **Alert Rules**.
4. Tune the "Slow query rate" rule to `> 10` per minute.

**Why use a rate, not a count?** A bulk migration can produce thousands of legitimate slow queries in seconds. A sustained per-minute rate catches real problems.

---

## Recipe 3 — Monitor MySQL replication lag

**Goal:** Open an incident if a replica falls more than 30 seconds behind the source for 2 minutes.

**Steps:**

1. Add a `mysql` check pointing at the **replica's** DSN (separate env, separate check from the source).
2. Open **Alert Rules → Replica lag**. Set threshold to `30` seconds, window to `120` seconds.
3. Verify: stop replication on the replica (`STOP REPLICA`). After ~3 minutes the incident fires.

**Tuning:** different workloads tolerate different lag. Read-heavy analytics replicas may accept 60s; financial replicas should be sub-second.

---

## Recipe 4 — Monitor deadlocks and lock waits

**Goal:** Catch a wave of deadlocks before users feel them.

**Steps:**

1. With the `mysql` check in place, go to **Alert Rules**.
2. Enable and tune:
   - "Deadlocks" → `> 0` in any 1-minute window (deadlocks are always worth knowing about).
   - "Locks waited" → `> 50` per minute (tune to your normal traffic).
   - "Lock wait time" → average wait `> 1` second per minute.

**On firing:** open the incident, then the attached MySQL snapshot. The snapshot includes the process list at the moment the rule fired — that is where you find the blocking transaction.

---

## Recipe 5 — Monitor a custom SQL query result

**Goal:** Alert when an arbitrary SQL query returns "bad". For example, "are there any rows in `payments_failed_today`?".

**Steps:**

1. **Checks → Add Check**.
2. Fill:
   - Type: `command`
   - Server: the host that has the `mysql` CLI installed
   - Command:
     ```
     mysql -h db.example.com -u readonly -p"$DBPASS" -BNe "SELECT COUNT(*) FROM payments_failed_today;" prod
     ```
   - Expected exit code: `0`
   - Expected output substring: `0`  *(passes only when count is zero)*
   - Interval: `300` seconds
3. Make sure `DBPASS` is set in the server environment (not the check) so the password is not stored.
4. Save.

**For Postgres** swap `mysql` for `psql -tAc`. The pattern is identical.

**Limitation:** this is a command check, not a native MySQL check. You will not get rich query metrics, only the pass/fail.

---

## Common Tuning Notes For Database Checks

- **Read-only user.** Never give HealthOps write privileges. It does not need them.
- **One check per DSN.** Each MySQL check runs its own sampler. Do not create dozens of overlapping checks against the same database.
- **Beware of `SHOW PROCESSLIST` cost** on huge servers. The default sampler interval (60s) is fine; do not drop below 30s.
- **Snapshots are gold during post-mortem.** Keep snapshot retention long enough that you can investigate week-old incidents, or archive them externally.
