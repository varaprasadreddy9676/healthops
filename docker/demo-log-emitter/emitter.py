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
SOURCE = os.getenv("LOG_SOURCE", "checkout-api")

FAMILIES = [
    {
        "level": "error",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "payment authorization timeout after 2500ms for order {order}",
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
        "level": "info",
        "source": "checkout-api",
        "server": "linux-server-1",
        "message": "checkout completed in {duration}ms for order {order}",
        "stackTrace": "",
        "tags": ["demo", "checkout"],
        "meta": {"service": "checkout"},
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
        duration=random.choice([318, 427, 941, 1330, 2440, 3100]),
        tenant=random.choice(["alpha", "bravo", "charlie"]),
    )


def build_batch(index):
    entries = []
    now = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    for offset, family in enumerate(random.sample(FAMILIES, k=min(4, len(FAMILIES)))):
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
