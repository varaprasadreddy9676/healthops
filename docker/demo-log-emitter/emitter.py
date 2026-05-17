#!/usr/bin/env python3
import json
import os
import random
import time
import urllib.error
import urllib.request


HEALTHOPS_URL = os.getenv("HEALTHOPS_URL", "http://healthops:8080").rstrip("/")
USERNAME = os.getenv("HEALTHOPS_USERNAME", "admin")
PASSWORD = os.getenv("HEALTHOPS_PASSWORD", "healthops-demo-admin")
BATCH_INTERVAL_SECONDS = int(os.getenv("BATCH_INTERVAL_SECONDS", "15"))
BATCH_SIZE = int(os.getenv("BATCH_SIZE", "5"))
SOURCE = os.getenv("LOG_SOURCE", "checkout-api")

# Real-world log families spanning application errors, database, security,
# infrastructure, JVM/runtime, distributed systems, and routine ops noise.
# build_batch() samples BATCH_SIZE entries per cycle from this pool.
FAMILIES = [
    # --- application / business errors ---
    {
        "level": "error",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "payment authorization timeout after {duration}ms for order {order}",
        "stackTrace": "TimeoutError: payment authorization timeout\n    at authorize (/srv/app/payments.js:88:13)\n    at checkout (/srv/app/checkout.js:42:9)",
        "tags": ["demo", "payments", "timeout"],
        "meta": {"service": "checkout", "dependency": "payment-gateway"},
    },
    {
        "level": "error",
        "source": "api-gateway",
        "server": "linux-server-1",
        "message": "upstream demo-api returned 503 for /checkout request_id={request_id}",
        "stackTrace": "",
        "tags": ["demo", "api", "upstream"],
        "meta": {"service": "gateway", "upstream": "demo-api"},
    },
    {
        "level": "error",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "circuit breaker OPEN for payment-gateway after 5 consecutive failures (last error: {duration}ms timeout)",
        "stackTrace": "",
        "tags": ["demo", "resilience", "circuit-breaker"],
        "meta": {"service": "checkout", "breaker": "payment-gateway", "state": "open"},
    },
    {
        "level": "warn",
        "source": "api-gateway",
        "server": "linux-server-1",
        "message": "rate limit triggered: 429 Too Many Requests from 198.51.100.{octet} (limit=1000/min)",
        "stackTrace": "",
        "tags": ["demo", "api", "rate-limit"],
        "meta": {"service": "gateway", "remoteIp": "198.51.100.0/24"},
    },
    {
        "level": "info",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "checkout completed in {duration}ms for order {order}",
        "stackTrace": "",
        "tags": ["demo", "checkout"],
        "meta": {"service": "checkout"},
    },
    # --- database / persistence ---
    {
        "level": "warn",
        "source": "mysql",
        "server": "mysql",
        "message": "slow query detected: SELECT COUNT(*) FROM demo_orders WHERE description LIKE '%card%' took {duration}ms",
        "stackTrace": "",
        "tags": ["demo", "mysql", "slow-query"],
        "meta": {"service": "mysql", "table": "demo_orders"},
    },
    {
        "level": "error",
        "source": "worker",
        "server": "linux-server-2",
        "message": "job reconciliation failed for tenant demo-{tenant}: deadlock found when trying to get lock",
        "stackTrace": "OperationalError: deadlock found\n  File \"/srv/workers/reconcile.py\", line 144, in run\n  File \"/srv/workers/reconcile.py\", line 61, in update_batch",
        "tags": ["demo", "worker", "deadlock"],
        "meta": {"service": "worker", "queue": "reconciliation"},
    },
    {
        "level": "error",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "HikariCP connection pool exhausted: 30/30 active, 12 pending threads waiting >5000ms (db=checkout_prod)",
        "stackTrace": "SQLTransientConnectionException: HikariPool-1 - Connection is not available, request timed out after 5001ms\n    at com.zaxxer.hikari.pool.HikariPool.createTimeoutException(HikariPool.java:696)\n    at com.zaxxer.hikari.pool.HikariPool.getConnection(HikariPool.java:197)",
        "tags": ["demo", "db", "pool-exhausted"],
        "meta": {"service": "checkout", "pool": "checkout_prod", "active": 30, "max": 30},
    },
    # --- security ---
    {
        "level": "warn",
        "source": "sshd",
        "server": "linux-server-2",
        "message": "Failed password for invalid user admin from 203.0.113.{octet} port {port} ssh2",
        "stackTrace": "",
        "tags": ["demo", "security", "sshd", "auth-failure"],
        "meta": {"service": "sshd", "remoteIp": "203.0.113.0/24"},
    },
    {
        "level": "error",
        "source": "auth-service",
        "server": "linux-server-1",
        "message": "JWT signature verification failed for request_id={request_id} (alg=HS256, kid=rotated-2024-q4)",
        "stackTrace": "JsonWebTokenError: invalid signature\n    at /srv/app/auth/jwt.js:55:22",
        "tags": ["demo", "security", "auth", "jwt"],
        "meta": {"service": "auth", "reason": "signature_invalid"},
    },
    # --- infrastructure / runtime ---
    {
        "level": "warn",
        "source": "jvm",
        "server": "linux-server-2",
        "message": "GC pause exceeded threshold: G1 Young Generation took {duration}ms (heap before=3.8G after=1.2G)",
        "stackTrace": "",
        "tags": ["demo", "jvm", "gc"],
        "meta": {"service": "event-consumer", "collector": "G1", "phase": "young"},
    },
    {
        "level": "error",
        "source": "kubelet",
        "server": "linux-server-2",
        "message": "Container event-consumer-7b9f4 in pod events-prod/event-consumer-deploy-xyz was OOMKilled (limit=2Gi rss=2.1Gi)",
        "stackTrace": "",
        "tags": ["demo", "k8s", "oom-killed"],
        "meta": {"service": "kubelet", "pod": "event-consumer-deploy-xyz", "namespace": "events-prod"},
    },
    {
        "level": "error",
        "source": "kubelet",
        "server": "linux-server-2",
        "message": "Pod payments-api-5d8c restarted 6 times in 10m (CrashLoopBackOff, last exit code 137)",
        "stackTrace": "",
        "tags": ["demo", "k8s", "crashloop"],
        "meta": {"service": "kubelet", "pod": "payments-api-5d8c", "restartCount": 6},
    },
    {
        "level": "error",
        "source": "kernel",
        "server": "linux-server-1",
        "message": "EXT4-fs warning: no space left on device /var/lib/docker (95% used, 250MB free of 50GB)",
        "stackTrace": "",
        "tags": ["demo", "disk", "out-of-space"],
        "meta": {"service": "kernel", "mount": "/var/lib/docker", "usagePercent": 95},
    },
    {
        "level": "warn",
        "source": "nginx",
        "server": "linux-server-1",
        "message": "SSL certificate for api.demo.example.com will expire in 7 days (issued by Let's Encrypt R3)",
        "stackTrace": "",
        "tags": ["demo", "tls", "cert-expiry"],
        "meta": {"service": "nginx", "issuer": "Let's Encrypt", "daysRemaining": 7},
    },
    # --- distributed systems / dependencies ---
    {
        "level": "error",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "redis ECONNREFUSED at cache-primary:6379 — falling back to direct DB read (request_id={request_id})",
        "stackTrace": "Error: connect ECONNREFUSED 10.0.5.12:6379\n    at TCPConnectWrap.afterConnect [as oncomplete] (node:net:1494:16)",
        "tags": ["demo", "cache", "redis", "connection-refused"],
        "meta": {"service": "checkout", "dependency": "redis", "endpoint": "cache-primary:6379"},
    },
    {
        "level": "error",
        "source": "worker",
        "server": "linux-server-2",
        "message": "DNS resolution failed for billing.internal.svc.cluster.local after 5 retries (NXDOMAIN)",
        "stackTrace": "",
        "tags": ["demo", "dns", "network"],
        "meta": {"service": "worker", "host": "billing.internal.svc.cluster.local"},
    },
    # --- structured / observability ---
    {
        "level": "info",
        "source": "api-gateway",
        "server": "linux-server-1",
        "message": "GET /api/v1/orders 200 {duration}ms request_id={request_id} trace_id=4bf92f3577b34da6a3ce929d0e0e4736 span_id=00f067aa0ba902b7",
        "stackTrace": "",
        "tags": ["demo", "access-log", "trace"],
        "meta": {"service": "gateway", "method": "GET", "path": "/api/v1/orders"},
    },
    {
        "level": "info",
        "source": "audit",
        "server": "linux-server-1",
        "message": "user.role.changed actor=admin@example.com target=user-{tenant} from=member to=admin",
        "stackTrace": "",
        "tags": ["demo", "audit", "compliance"],
        "meta": {"service": "audit", "event": "user.role.changed", "actor": "admin@example.com"},
    },
    {
        "level": "warn",
        "source": "feature-flags",
        "server": "linux-server-1",
        "message": "feature flag evaluation timeout: defaulting to OFF for flag=checkout_v2_new_pricing (provider=launchdarkly took >250ms)",
        "stackTrace": "",
        "tags": ["demo", "feature-flag", "timeout"],
        "meta": {"service": "checkout", "flag": "checkout_v2_new_pricing"},
    },
]


def request_json(method, path, payload=None, token=None, timeout=10):
    body = None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")

    req = urllib.request.Request(f"{HEALTHOPS_URL}{path}", data=body, method=method, headers=headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8"))


def login():
    body = request_json("POST", "/api/v1/auth/login", {"username": USERNAME, "password": PASSWORD}, timeout=5)
    token = body.get("data", {}).get("token")
    if not token:
        raise RuntimeError("login response did not include a token")
    return token


def render(template, index):
    return template.format(
        order=100000 + index,
        request_id=f"req-{int(time.time())}-{index}",
        duration=random.choice([318, 427, 941, 1330, 2440, 3100, 5012]),
        tenant=random.choice(["alpha", "bravo", "charlie", "delta"]),
        octet=random.randint(2, 254),
        port=random.randint(40000, 65000),
    )


def build_batch(index):
    entries = []
    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    sample_size = min(BATCH_SIZE, len(FAMILIES))
    for offset, family in enumerate(random.sample(FAMILIES, k=sample_size)):
        entry = {
            "timestamp": now,
            "level": family["level"],
            "message": render(family["message"], index + offset),
            "source": family.get("source", SOURCE),
            "server": family["server"],
            "stackTrace": family["stackTrace"],
            "tags": family["tags"],
            "meta": family["meta"],
        }
        entries.append(entry)
    return {"entries": entries}


def main():
    token = None
    index = 0
    last_success = 0

    while True:
        try:
            if token is None:
                token = login()
                print("demo-log-emitter authenticated", flush=True)

            payload = build_batch(index)
            result = request_json("POST", "/api/v1/logs/ingest", payload, token=token, timeout=10)
            ingested = result.get("data", {}).get("ingested", 0)
            index += ingested
            last_success = time.time()
            if index == ingested or index % 20 == 0:
                print(f"demo-log-emitter ingested={index}", flush=True)

        except urllib.error.HTTPError as err:
            if err.code in (401, 403):
                token = None
            print(f"demo-log-emitter waiting for HealthOps ({err.code})", flush=True)
        except Exception as err:
            if time.time() - last_success > 60:
                print(f"demo-log-emitter waiting: {err}", flush=True)

        time.sleep(BATCH_INTERVAL_SECONDS)


if __name__ == "__main__":
    main()
