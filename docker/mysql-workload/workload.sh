#!/usr/bin/env sh
set -eu

MYSQL_HOST="${MYSQL_HOST:-mysql}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-healthops}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-healthops123}"
MYSQL_DATABASE="${MYSQL_DATABASE:-healthops}"
WORKLOAD_INTERVAL_SECONDS="${WORKLOAD_INTERVAL_SECONDS:-8}"

export MYSQL_PWD="$MYSQL_PASSWORD"

echo "mysql-workload waiting for MySQL at ${MYSQL_HOST}:${MYSQL_PORT}"
until mysqladmin ping -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" --silent; do
  sleep 2
done

mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DATABASE" <<'SQL'
CREATE TABLE IF NOT EXISTS demo_customers (
  id INT AUTO_INCREMENT PRIMARY KEY,
  external_id VARCHAR(64) NOT NULL UNIQUE,
  name VARCHAR(160) NOT NULL,
  tier VARCHAR(32) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS demo_orders (
  id INT AUTO_INCREMENT PRIMARY KEY,
  order_ref VARCHAR(64) NOT NULL UNIQUE,
  customer_external_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  amount_cents INT NOT NULL,
  description TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS demo_audit_events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  event_type VARCHAR(80) NOT NULL,
  actor VARCHAR(120) NOT NULL,
  payload JSON NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
SQL

i=1
while [ "$i" -le 150 ]; do
  tier="standard"
  if [ $((i % 10)) -eq 0 ]; then tier="enterprise"; fi
  mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DATABASE" \
    -e "INSERT IGNORE INTO demo_customers (external_id, name, tier) VALUES ('cust-${i}', 'Demo Customer ${i}', '${tier}');" >/dev/null
  i=$((i + 1))
done

echo "mysql-workload started"
tick=1
while true; do
  customer_id=$(( (tick % 150) + 1 ))
  status="paid"
  if [ $((tick % 9)) -eq 0 ]; then status="retrying"; fi
  if [ $((tick % 17)) -eq 0 ]; then status="failed"; fi

  mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" "$MYSQL_DATABASE" <<SQL >/dev/null
INSERT IGNORE INTO demo_orders (order_ref, customer_external_id, status, amount_cents, description)
VALUES (
  'order-${tick}',
  'cust-${customer_id}',
  '${status}',
  1000 + (${tick} % 9000),
  CONCAT('card checkout authorization path order=', ${tick}, ' status=', '${status}', ' payload=', REPEAT('x', 128))
);

INSERT INTO demo_audit_events (event_type, actor, payload)
VALUES (
  'checkout.${status}',
  'demo-worker',
  JSON_OBJECT('orderRef', 'order-${tick}', 'customer', 'cust-${customer_id}', 'attempt', ${tick})
);

SELECT COUNT(*) FROM demo_orders WHERE description LIKE '%card checkout authorization%';
SELECT status, COUNT(*) FROM demo_orders GROUP BY status ORDER BY COUNT(*) DESC;
SELECT SLEEP(0.05);
SQL

  tick=$((tick + 1))
  sleep "$WORKLOAD_INTERVAL_SECONDS"
done
