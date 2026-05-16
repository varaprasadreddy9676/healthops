#!/usr/bin/env python3
import json
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


PORT = int(os.getenv("PORT", "9200"))
MODEL = os.getenv("DEMO_AI_MODEL", "healthops-demo-ops-model")


def completion_for(messages):
    joined = "\n".join(str(m.get("content", "")) for m in messages).lower()
    is_rca = "root-cause analysis" in joined or "hypotheses" in joined or "correlated signals" in joined
    is_mysql = "mysql" in joined
    is_brief = "incident brief" in joined or "evidence citations" in joined or "likelycause" in joined

    if is_rca:
        payload = {
            "summary": "Demo RCA: checkout latency correlates with upstream payment timeouts and MySQL workload activity.",
            "hypotheses": [
                {
                    "rank": 1,
                    "title": "Payment dependency latency propagated to checkout",
                    "description": "The checkout synthetic checks slowed at the same time as payment authorization timeout log families appeared.",
                    "confidence": 0.86,
                    "category": "application",
                    "evidence": [
                        "Checkout API latency checks breached their warning threshold.",
                        "Log families include payment authorization timeouts from checkout-api.",
                        "The incident window overlaps with synthetic checkout degradation."
                    ],
                    "suggestion": "Inspect payment gateway latency, circuit-breaker thresholds, and retry/backoff settings before scaling checkout workers."
                },
                {
                    "rank": 2,
                    "title": "Database workload amplified request latency",
                    "description": "MySQL workload and slow-query log families are present in the same window, so database pressure may be a contributing factor.",
                    "confidence": 0.62,
                    "category": "database",
                    "evidence": [
                        "MySQL monitoring samples are present for the incident window.",
                        "Slow query families mention full scans on demo_orders.",
                        "Checkout requests depend on order persistence."
                    ],
                    "suggestion": "Review top MySQL digests and add indexes or query guards for demo_orders search paths."
                }
            ],
            "rootCause": "Payment dependency latency with possible database amplification.",
            "impact": "Checkout requests may be slow or intermittently fail.",
            "severity": "critical",
            "suggestions": [
                "Check payment gateway health and timeout budgets.",
                "Inspect checkout-api recent log families.",
                "Review MySQL top queries and connection pressure.",
                "Keep the incident open until the checkout synthetic check recovers."
            ],
            "confidence": "high"
        }
        return json.dumps(payload)

    if is_brief:
        payload = {
            "likelyCause": "The checkout API is degraded by the active demo scenario, with payment timeout logs and latency checks in the same incident window.",
            "impactSummary": "Synthetic checkout traffic is slow or failing, which represents degraded customer checkout behavior.",
            "nextActions": [
                "Open the incident timeline and identify the first failed checkout signal.",
                "Inspect checkout-api and payment-gateway log families for timeout frequency.",
                "Review MySQL workload samples for database amplification.",
                "Run scripts/demo-scenario.sh recover after validating the incident workflow."
            ],
            "evidenceCitations": [
                {
                    "category": "checks",
                    "description": "The demo-api-latency synthetic check breached its latency threshold."
                },
                {
                    "category": "logs",
                    "description": "Checkout and payment timeout log families were ingested during the same window."
                },
                {
                    "category": "mysql",
                    "description": "The demo workload can provide database context for possible latency amplification."
                }
            ]
        }
        return json.dumps(payload)

    if is_mysql:
        payload = {
            "summary": "Demo MySQL analysis: workload activity is visible with slow-query indicators.",
            "rootCause": "Unindexed demo_orders scans are increasing query cost.",
            "impact": "Database latency can slow checkout and reconciliation paths.",
            "severity": "warning",
            "urgency": "soon",
            "mysqlSpecific": {
                "queryAnalysis": "Review demo_orders LIKE scans and top digest rows.",
                "connectionAnalysis": "Connection pressure is not the primary signal in the demo baseline.",
                "capacityAnalysis": "Sustained workload should be tested with realistic production cardinality."
            },
            "suggestions": [
                "Add an index for common order lookup filters.",
                "Avoid broad LIKE scans on high-volume order text columns.",
                "Track slow query rate and rows examined per digest."
            ],
            "confidence": "medium"
        }
        return json.dumps(payload)

    payload = {
        "summary": "Demo AI analysis: HealthOps correlated check failures, logs, and infrastructure signals.",
        "rootCause": "The most likely cause is the active demo scenario affecting checkout-api behavior.",
        "impact": "Synthetic checkout monitoring detected degraded customer-facing behavior.",
        "severity": "critical",
        "suggestions": [
            "Open the incident timeline and confirm which check breached first.",
            "Review active log families for checkout-api and api-gateway.",
            "Run scripts/demo-scenario.sh recover after testing the workflow."
        ],
        "additionalDataNeeded": [],
        "confidence": "high"
    }
    return json.dumps(payload)


class Handler(BaseHTTPRequestHandler):
    server_version = "HealthOpsDemoAI/1.0"

    def log_message(self, _fmt, *_args):
        return

    def do_GET(self):
        if self.path in ("/health", "/v1/models"):
            if self.path == "/health":
                return self.write_json(200, {"status": "ok", "model": MODEL})
            return self.write_json(200, {"object": "list", "data": [{"id": MODEL, "object": "model"}]})
        return self.write_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            return self.write_json(404, {"error": "not found"})

        length = int(self.headers.get("content-length", "0") or "0")
        raw = self.rfile.read(length) if length else b"{}"
        try:
            body = json.loads(raw.decode("utf-8"))
        except Exception:
            return self.write_json(400, {"error": {"message": "invalid json"}})

        content = completion_for(body.get("messages", []))
        return self.write_json(200, {
            "id": f"chatcmpl-demo-{int(time.time() * 1000)}",
            "object": "chat.completion",
            "created": int(time.time()),
            "model": body.get("model", MODEL),
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": content},
                    "finish_reason": "stop"
                }
            ],
            "usage": {"prompt_tokens": 120, "completion_tokens": 180, "total_tokens": 300}
        })

    def write_json(self, status, payload):
        data = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main():
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    print(f"demo-ai-provider listening on :{PORT}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
