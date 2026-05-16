#!/usr/bin/env python3
import json
import os
import random
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse


PORT = int(os.getenv("PORT", "9100"))

state_lock = threading.Lock()
state = {
    "fail": False,
    "slow": False,
    "flaky": False,
    "requests": 0,
    "startedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
}


def snapshot():
    with state_lock:
        return dict(state)


def set_flag(name, enabled):
    with state_lock:
        state[name] = enabled
        return dict(state)


def next_request():
    with state_lock:
        state["requests"] += 1
        return state["requests"], dict(state)


class Handler(BaseHTTPRequestHandler):
    server_version = "HealthOpsDemoAPI/1.0"

    def log_message(self, _fmt, *_args):
        return

    def do_GET(self):
        self.route()

    def do_POST(self):
        self.route()

    def route(self):
        parsed = urlparse(self.path)
        path = parsed.path

        if path == "/":
            return self.write_json(200, {
                "service": "checkout-api",
                "status": "ok",
                "endpoints": ["/health", "/checkout", "/slow", "/status", "/toggle/fail", "/toggle/slow", "/toggle/flaky", "/recover"],
            })

        if path == "/status":
            return self.write_json(200, snapshot())

        if path == "/recover":
            with state_lock:
                state["fail"] = False
                state["slow"] = False
                state["flaky"] = False
                current = dict(state)
            return self.write_json(200, {"status": "recovered", "state": current})

        if path.startswith("/toggle/"):
            name = path.rsplit("/", 1)[-1]
            if name not in ("fail", "slow", "flaky"):
                return self.write_json(404, {"error": "unknown scenario"})
            enabled = parse_qs(parsed.query).get("enabled", ["true"])[0].lower() not in ("0", "false", "no", "off")
            current = set_flag(name, enabled)
            return self.write_json(200, {"status": "updated", "state": current})

        request_no, current = next_request()

        if path == "/health":
            if current["fail"]:
                return self.write_json(503, {"status": "down", "reason": "demo fail scenario enabled"})
            return self.write_json(200, {"status": "ok", "service": "checkout-api", "request": request_no})

        if path == "/slow":
            delay = 1.8 if current["slow"] else 0.12
            time.sleep(delay)
            return self.write_json(200, {"status": "ok", "delayMs": int(delay * 1000), "request": request_no})

        if path == "/checkout":
            if current["fail"]:
                return self.write_json(503, {"status": "down", "message": "payment dependency unavailable"})
            if current["slow"]:
                time.sleep(1.4)
            if current["flaky"] and (request_no % 3 == 0 or random.random() < 0.15):
                return self.write_json(500, {"status": "error", "message": "intermittent payment authorization failure"})
            return self.write_json(200, {"status": "accepted", "orderId": f"demo-{request_no}", "request": request_no})

        if path == "/error":
            return self.write_json(500, {"status": "error", "message": "intentional demo error"})

        if path == "/metrics":
            body = (
                "# HELP demo_api_requests_total Requests handled by the demo API\n"
                "# TYPE demo_api_requests_total counter\n"
                f"demo_api_requests_total {snapshot()['requests']}\n"
            )
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; version=0.0.4")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body.encode("utf-8"))
            return

        self.write_json(404, {"error": "not found"})

    def write_json(self, status, payload):
        body = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main():
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    print(f"demo-api listening on :{PORT}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
