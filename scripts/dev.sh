#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Job control gives each watcher its own process group. That lets cleanup stop
# child processes such as Air's temporary `go build` as well as the watchers.
set -m

pids=()

cleanup() {
  trap - INT TERM EXIT
  for pid in "${pids[@]}"; do
    kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
  done
  sleep 0.2
  for pid in "${pids[@]}"; do
    kill -KILL -- "-$pid" 2>/dev/null || true
  done
  wait "${pids[@]}" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

./.tools/templ generate --watch &
pids+=("$!")

# `always` keeps Tailwind watching even when stdin is not a terminal.
./.tools/tailwindcss -i ./styles/input.css -o ./static/css/app.css --watch=always &
pids+=("$!")

./.tools/air -c .air.toml &
pids+=("$!")

# Stop everything if any watcher or the app exits.
set +e
wait -n
status=$?
set -e
exit "$status"
