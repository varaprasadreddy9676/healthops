#!/bin/bash
set -e

# Start SSH daemon (daemonized, fork+detach)
/usr/sbin/sshd

# Start cron daemon (daemonized)
cron

# Start nginx (daemonized master process)
nginx

# Start Python HTTP server on configured port (default 8000)
HTTP_PORT=${HTTP_PORT:-8000}
python3 -m http.server "$HTTP_PORT" --directory /tmp > /tmp/http.log 2>&1 &
HTTP_PID=$!

echo "=== Linux Demo Server Started ==="
echo "  SSH:    port 22"
echo "  Nginx:  port 80"
echo "  Python: port $HTTP_PORT (pid $HTTP_PID)"
echo "  Cron:   running"
echo "================================="

# Supervise critical processes. Exit non-zero if any dies so Docker can restart
# the container instead of running silently broken.
while true; do
  if ! pgrep -x sshd >/dev/null; then
    echo "FATAL: sshd is no longer running" >&2
    exit 1
  fi
  if ! pgrep -x cron >/dev/null; then
    echo "FATAL: cron is no longer running" >&2
    exit 1
  fi
  if ! pgrep -x nginx >/dev/null; then
    echo "FATAL: nginx is no longer running" >&2
    exit 1
  fi
  if ! kill -0 "$HTTP_PID" 2>/dev/null; then
    echo "FATAL: python http.server (pid $HTTP_PID) is no longer running" >&2
    exit 1
  fi
  sleep 10
done
