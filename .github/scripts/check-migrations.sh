#!/usr/bin/env bash
# 迁移检查（CI 的 migrations 任务）：在空库上起 server 跑 public 迁移，再对槽位 dlpv1
# 执行两次 tenant reset，比较两次的结构与数据；最后重启 server，确认租户可用。
#
#   DATABASE_URL=postgres://…/空库 BIN=bin .github/scripts/check-migrations.sh
#
# BIN 目录里要有 server 和 tenant（go build -o bin/ ./api/cmd/...）。需要 psql 和 curl。
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL 未设置}"
BIN=${BIN:-bin}
ADDR=${ADDR:-127.0.0.1:18090}
SLOT=dlpv1
export TENANT_TEST_PASSWORD=${TENANT_TEST_PASSWORD:-migration-check}
work=$(mktemp -d)
server_pid=""
trap '[ -n "$server_pid" ] && kill "$server_pid" 2>/dev/null; rm -rf "$work"' EXIT

q() { psql "$DATABASE_URL" -X -q -At -v ON_ERROR_STOP=1 -c "$1"; }

start_server() {
  ADDR=$ADDR "$BIN/server" >"$work/server.log" 2>&1 &
  server_pid=$!
  for _ in $(seq 1 50); do
    if curl -fsS "http://$ADDR/healthz" >/dev/null 2>&1; then return 0; fi
    if ! kill -0 "$server_pid" 2>/dev/null; then break; fi
    sleep 0.2
  done
  echo "server 没有就绪：" >&2
  cat "$work/server.log" >&2
  exit 1
}

stop_server() {
  kill "$server_pid"
  wait "$server_pid" || true
  server_pid=""
}

fingerprint() {
  {
    echo "== columns"
    q "select table_name, column_name, data_type, is_nullable, coalesce(column_default, '')
       from information_schema.columns where table_schema = '$SLOT' order by 1, 2"
    echo "== indexes"
    q "select indexname, indexdef from pg_indexes where schemaname = '$SLOT' order by 1"
    echo "== constraints"
    q "select conrelid::regclass::text, conname, pg_get_constraintdef(oid)
       from pg_constraint where connamespace = '$SLOT'::regnamespace order by 1, 2"
    echo "== sequences"
    q "select sequencename, coalesce(last_value, 0) from pg_sequences where schemaname = '$SLOT' order by 1"
    echo "== goose"
    q "select version_id, is_applied from $SLOT.goose_db_version order by id"
    for t in job report crew_hours report_note; do
      echo "== $t"
      q "select * from $SLOT.$t order by 1, 2"
    done
    echo "== accounts"
    q "select email, display_name, role, disabled from public.app_user where tenant = '$SLOT' order by email"
  } >"$1"
}

if [ "$(q "select count(*) from pg_tables where schemaname not in ('pg_catalog', 'information_schema')")" != 0 ]; then
  echo "DATABASE_URL 指向的库不是空库" >&2
  exit 1
fi

echo "1. 空库上启动 server（public 迁移）"
start_server
stop_server
echo "   public 迁移版本：$(q "select max(version_id) from public.goose_db_version where is_applied")"

echo "2. tenant reset 两次并比较"
"$BIN/tenant" reset -tenant "$SLOT" -snapshot "${SNAPSHOT:-api/testdata/golden}" >/dev/null
fingerprint "$work/first"
"$BIN/tenant" reset -tenant "$SLOT" -snapshot "${SNAPSHOT:-api/testdata/golden}" >/dev/null
fingerprint "$work/second"
if ! diff -u "$work/first" "$work/second"; then
  echo "两次 reset 的结果不同" >&2
  exit 1
fi
echo "   一致（$(wc -l <"$work/first" | tr -d ' ') 行指纹）"

echo "3. 重启 server，用测试账号读取台账"
start_server
jar="$work/cookies"
curl -fsS -c "$jar" -H 'Content-Type: application/json' \
  -d "{\"email\":\"admin@$SLOT.test\",\"password\":\"$TENANT_TEST_PASSWORD\"}" "http://$ADDR/api/login" >/dev/null
rows=$(curl -fsS -b "$jar" "http://$ADDR/api/reports" | grep -o '"id":' | wc -l | tr -d ' ')
want=$(q "select count(*) from $SLOT.report")
stop_server
if [ "$rows" != "$want" ]; then
  echo "台账行数 $rows，数据库里 $want 份日报" >&2
  exit 1
fi
echo "   台账 $rows 行"
echo "迁移检查通过"
