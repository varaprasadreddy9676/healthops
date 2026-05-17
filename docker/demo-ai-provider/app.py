#!/usr/bin/env python3
import json
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


PORT = int(os.getenv("PORT", "9200"))
MODEL = os.getenv("DEMO_AI_MODEL", "healthops-demo-ops-model")


def completion_for(messages):
    joined = "\n".join(str(m.get("content", "")) for m in messages).lower()
    user_joined = "\n".join(str(m.get("content", "")) for m in messages if m.get("role") == "user").lower()
    is_log_categorization = "categorize this error family" in joined or "categorizing error log patterns" in joined
    is_automation = "suggest remediation actions" in joined or "suggest specific remediation actions" in joined
    is_rca = "root-cause analysis" in joined or "hypotheses" in joined or "correlated signals" in joined
    is_assistant = "you are healthops assistant" in joined or "current system telemetry" in joined
    is_mysql = "mysql" in user_joined or "database" in user_joined
    is_brief = "incident brief" in joined or "evidence citations" in joined or "likelycause" in joined

    if is_log_categorization:
        family_text = user_joined
        category = "unknown"
        severity = "warning"
        summary = "Recurring log pattern needs operator review."

        if any(term in family_text for term in ("access denied", "database auth", "mysql auth", "using password", "authentication failed")):
            category = "db_auth"
            severity = "critical"
            summary = "Database authentication failures are recurring and may block dependent services."
        elif any(term in family_text for term in ("failed password", "jwt signature", "invalid signature", "brute-force", "break-in")):
            category = "security"
            severity = "critical"
            summary = "Security authentication failures are recurring and should be reviewed."
        elif any(term in family_text for term in ("permission denied", "forbidden", "unauthorized")):
            category = "permission"
            severity = "warning"
            summary = "Authorization or permission failures are recurring."
        elif any(term in family_text for term in ("slow query", "rows examined", "full table scan", "select *", "demo_orders")):
            category = "slow_query"
            severity = "warning"
            summary = "Slow query patterns suggest inefficient database access or missing indexes."
        elif any(term in family_text for term in ("deadlock", "lock wait", "database lock")):
            category = "database"
            severity = "critical"
            summary = "Database locking failures are recurring and may block background work."
        elif any(term in family_text for term in ("connection refused", "connection reset", "no route to host", "dns", "nxdomain", "unreachable", "econnrefused")):
            category = "network"
            severity = "critical"
            summary = "Network reachability failures are recurring between monitored services."
        elif any(term in family_text for term in ("too many connections", "thread pool", "threads_running", "pool exhausted", "hikaricp")):
            category = "thread_exhaustion"
            severity = "critical"
            summary = "Connection or worker pool exhaustion is recurring and can cascade into outages."
        elif any(term in family_text for term in ("out of memory", "oom", "oomkilled", "heap", "memory allocation", "exit code 137")):
            category = "memory"
            severity = "critical"
            summary = "Memory pressure or allocation failures are recurring."
        elif any(term in family_text for term in ("no space left", "disk full", "disk pressure", "i/o error", "enospc", "errno 28")):
            category = "disk_io"
            severity = "critical"
            summary = "Disk capacity or I/O failures are recurring on the affected host."
        elif any(term in family_text for term in ("rate limit", "too many requests", " 429 ")):
            category = "rate_limit"
            severity = "warning"
            summary = "Rate limiting is recurring and may indicate traffic spikes or an overly tight quota."
        elif any(term in family_text for term in ("get /", "post /", " 200 ", " 201 ", " 204 ")):
            category = "access_log"
            severity = "info"
            summary = "HTTP access logs are recurring routine request telemetry."
        elif any(term in family_text for term in ("audit", "role.changed", "user.role")):
            category = "audit"
            severity = "info"
            summary = "Audit events are recurring and may require compliance review."
        elif any(term in family_text for term in ("missing env", "config", "configuration", "invalid setting", "feature flag", "launchdarkly")):
            category = "config"
            severity = "warning"
            summary = "Configuration errors are recurring and should be fixed at deploy/runtime config."
        elif any(term in family_text for term in ("exception", "traceback", "panic", "null pointer", "nil pointer", "typeerror", "valueerror", "crashloopbackoff")):
            category = "app_bug"
            severity = "critical" if any(term in family_text for term in ("panic", "crashloopbackoff")) else "warning"
            summary = "Application exceptions are recurring and likely need a code or input-handling fix."
        elif any(term in family_text for term in ("checkout completed", "request completed")):
            category = "application"
            severity = "info"
            summary = "Application activity logs are recurring normal workflow telemetry."
        elif any(term in family_text for term in ("timeout", "timed out", "deadline exceeded", "context deadline")):
            category = "timeout"
            severity = "critical" if "checkout" in family_text or "payment" in family_text else "warning"
            summary = "Timeouts are recurring and may indicate a slow upstream dependency or overloaded service."

        return json.dumps({
            "category": category,
            "summary": summary,
            "severity": severity
        })

    if is_automation:
        actions = []

        if "python http" in joined or "http.server" in joined:
            actions.append({
                "type": "custom",
                "title": "Inspect missing Python HTTP demo process",
                "description": "Confirm whether the demo HTTP process is intentionally stopped before restarting it.",
                "risk": "low",
                "command": "ps aux | grep '[h]ttp.server' || python3 -m http.server 8080",
                "reason": "The active incident says the expected http.server process is not running."
            })

        if "stale log" in joined or "log heartbeat stale" in joined:
            actions.append({
                "type": "custom",
                "title": "Verify stale log writer",
                "description": "Check whether the monitored application is still writing to the expected log path.",
                "risk": "low",
                "command": "tail -n 50 /app/data/stale-demo.log && stat /app/data/stale-demo.log",
                "reason": "The stale-log check is critical because the file timestamp is older than the freshness threshold."
            })

        if "heartbeat" in joined or "no heartbeat ping" in joined:
            actions.append({
                "type": "custom",
                "title": "Install heartbeat ping in the scheduled job",
                "description": "Add the generated ping URL to the cron job or disable the QA heartbeat if it is only a test fixture.",
                "risk": "medium",
                "command": "curl -fsS \"$HEALTHOPS_HEARTBEAT_URL\"",
                "reason": "The heartbeat incident is open because HealthOps has not received a ping for that check."
            })

        if not actions:
            actions.append({
                "type": "custom",
                "title": "Review current critical checks",
                "description": "Open the failing checks and compare latest result messages before taking infrastructure action.",
                "risk": "low",
                "command": "",
                "reason": "The context did not identify one specific remediation target, so the safest next step is investigation."
            })

        return json.dumps(actions[:3])

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

    if is_assistant:
        open_items = []
        if "python http on server 1" in joined:
            open_items.append("Python HTTP on Server 1 is critical because the `http.server` process is not running.")
        if "python http on server 2" in joined:
            open_items.append("Python HTTP on Server 2 is critical because the `http.server` process is not running.")
        if "demo stale log" in joined:
            open_items.append("Demo Stale Log is critical because the log heartbeat is stale.")
        if "qa cron heartbeat" in joined or "qa-heartbeat" in joined:
            open_items.append("QA Cron Heartbeat is critical because no recent heartbeat ping is present.")

        if not open_items:
            return (
                "I do not see open critical incidents in the provided telemetry. "
                "Review the unhealthy check references first if the UI is showing a different state."
            )

        bullets = "\n".join(f"- {item}" for item in open_items)
        return (
            "From the telemetry context, the current critical items are:\n\n"
            f"{bullets}\n\n"
            "Check these first:\n"
            "1. Confirm whether the Python HTTP demo processes are intentionally stopped or should be restarted.\n"
            "2. For `Demo Stale Log`, verify the monitored log source is still writing fresh entries.\n"
            "3. For `QA Cron Heartbeat`, add the generated ping URL to the cron/job or disable the test heartbeat if it is only a QA fixture.\n"
            "4. Keep checkout/API checks under observation because recent demo scenarios can create resolved incidents and noisy log families."
        )

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

    summary = "Demo AI analysis: the incident matches the active demo scenario and should be validated against check results and logs."
    root_cause = "The most likely cause is the active demo scenario affecting monitored services."
    impact = "Monitoring detected degraded behavior for the affected check."
    suggestions = [
        "Open the incident timeline and confirm which check breached first.",
        "Compare the latest check result with related log families.",
        "Run scripts/demo-scenario.sh recover after testing the workflow."
    ]

    if "http.server" in user_joined or "python http" in user_joined:
        summary = "Demo AI analysis: the expected Python HTTP demo process is not running on the monitored Linux host."
        root_cause = "The process monitor could not find `http.server`, which usually means the demo process was stopped or never started."
        impact = "Process checks for the Python demo service remain critical until the service is started or the check is disabled."
        suggestions = [
            "Confirm whether the Python HTTP demo service is expected to be running.",
            "Inspect the host process list for `http.server` before restarting anything.",
            "Start the demo HTTP service or disable the fixture check if it is intentionally stopped."
        ]
    elif "stale log" in user_joined or "log heartbeat stale" in user_joined:
        summary = "Demo AI analysis: the monitored log file has not received fresh writes within its freshness window."
        root_cause = "The log writer may be stopped, misconfigured, or writing to a different path."
        impact = "Log freshness monitoring is critical because HealthOps cannot trust that application activity is still being observed."
        suggestions = [
            "Check the monitored log file timestamp and latest lines.",
            "Verify the application is writing to the configured path.",
            "Restart or repair the log emitter only after confirming the expected source."
        ]
    elif "heartbeat" in user_joined or "no heartbeat ping" in user_joined:
        summary = "Demo AI analysis: the heartbeat check is open because HealthOps has not received a recent ping."
        root_cause = "The scheduled job or test fixture is not calling the generated heartbeat URL."
        impact = "HealthOps treats the monitored job as missing or stalled until a heartbeat arrives."
        suggestions = [
            "Copy the heartbeat ping URL into the scheduled job.",
            "Run one manual ping to confirm the token and network path work.",
            "Disable the QA heartbeat if it is only a temporary test fixture."
        ]
    elif "unexpected status code" in user_joined or "status code 503" in user_joined:
        summary = "Demo AI analysis: the synthetic HTTP check received a 503 response from the monitored endpoint."
        root_cause = "The demo API was temporarily unavailable or intentionally forced into an error scenario."
        impact = "Customer-facing synthetic checks can fail until the endpoint returns a healthy status again."
        suggestions = [
            "Inspect the failing endpoint and recent API logs.",
            "Confirm whether a demo fault scenario is still active.",
            "Recover the scenario and keep the check under observation until it is stable."
        ]
    elif "dial tcp: lookup" in user_joined or "no such host" in user_joined:
        summary = "Demo AI analysis: the monitor could not resolve the target service name from inside the demo network."
        root_cause = "The target container or Docker DNS entry was temporarily unavailable during the check window."
        impact = "Network-dependent checks fail until service discovery and the target container are healthy again."
        suggestions = [
            "Confirm the target container is running and healthy.",
            "Check Docker network DNS resolution from the HealthOps container.",
            "Wait for startup ordering to settle, then rerun the affected checks."
        ]

    payload = {
        "summary": summary,
        "rootCause": root_cause,
        "impact": impact,
        "severity": "critical",
        "suggestions": suggestions,
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
