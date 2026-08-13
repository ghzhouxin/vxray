#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVER_DIR="$ROOT_DIR/server"
WEB_DIR="$ROOT_DIR/web"
OUTPUT_DIR="$ROOT_DIR/output"
DEBUG_BINARY="$OUTPUT_DIR/vxray-dev"
DEV_HOME="$HOME/.vxray-dev"
LOG_DIR="$DEV_HOME/logs"
DEBUG_PORT="10888"

PID_FILE="$DEV_HOME/vxray-dev.pid"
LOG_FILE="$LOG_DIR/vxray-dev.log"

usage() {
  cat <<EOF
Usage: ./scripts/dev.sh <command>

Commands:
  build        Build backend and frontend
  start        Build and start debug process in background
  stop         Stop the debug process
  restart      Rebuild and restart
  status       Show debug process status
  check        Check debug endpoints
  logs         Tail debug logs
  sudo-setup   Configure passwordless sudo for xray (one-time, for TUN mode)
EOF
}

build_all() {
  mkdir -p "$OUTPUT_DIR"
  (cd "$SERVER_DIR" && env GOCACHE=/tmp/go-build-cache go build -o "$DEBUG_BINARY" ./cmd)
  (cd "$WEB_DIR" && npm run build)
}

start_debug() {
  build_all
  mkdir -p "$LOG_DIR"
  stop_debug >/dev/null 2>&1 || true

  cd "$ROOT_DIR"
  VXRAY_HOME="$DEV_HOME" \
  VXRAY_WEB_ROOT="$WEB_DIR/dist" \
  VXRAY_SERVER_PORT="$DEBUG_PORT" \
  nohup "$DEBUG_BINARY" > "$LOG_FILE" 2>&1 & echo $! > "$PID_FILE"

  echo "Started vxray-dev (PID: $(cat "$PID_FILE"))"
  echo "  VXRAY_HOME=$DEV_HOME"
  echo "  http://127.0.0.1:$DEBUG_PORT"
}

stop_debug() {
  if [ -f "$PID_FILE" ]; then
    pid=$(cat "$PID_FILE")
    if kill -0 "$pid" 2>/dev/null && ps -p "$pid" -o comm= 2>/dev/null | grep -q "vxray"; then
      echo "Stopping vxray-dev (PID: $pid)..."
      kill "$pid"
      # Wait for graceful shutdown (also kills child xray processes)
      for i in $(seq 1 10); do
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.5
      done
    fi
    rm -f "$PID_FILE"
  else
    pid=$(lsof -i :"$DEBUG_PORT" -P 2>/dev/null | grep LISTEN | awk '{print $2}' | head -1)
    if [ -n "$pid" ]; then
      echo "Stopping process on port $DEBUG_PORT (PID: $pid)..."
      kill "$pid" 2>/dev/null || true
    else
      echo "No debug process running."
    fi
  fi
}

show_status() {
  if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    echo "vxray-dev: Running (PID: $(cat "$PID_FILE"))"
    echo "Log path: $LOG_FILE"
  else
    echo "vxray-dev: Stopped"
  fi
}

check() {
  curl -fsSI "http://127.0.0.1:$DEBUG_PORT/" \
  && curl -fsS "http://127.0.0.1:$DEBUG_PORT/api/xray/runtime"
}

show_logs() {
  mkdir -p "$LOG_DIR"
  tail -f "$LOG_FILE"
}

setup_sudo() {
  XRAY_PATH=$(which xray 2>/dev/null || true)
  if [ -z "$XRAY_PATH" ]; then
    echo "Error: xray not found in PATH"
    exit 1
  fi
  SUDOERS_LINE="$(whoami) ALL=(root) NOPASSWD: $XRAY_PATH, /bin/kill"
  SUDOERS_FILE="/etc/sudoers.d/vxray-dev"

  echo "This will create $SUDOERS_FILE with:"
  echo "  $SUDOERS_LINE"
  echo ""
  echo "(/bin/kill 用于 root xray 强杀兜底)"
  echo "You will be prompted for your password once."

  sudo tee "$SUDOERS_FILE" > /dev/null <<< "$SUDOERS_LINE"
  sudo chmod 0440 "$SUDOERS_FILE"
  echo "Done. sudo -n xray should now work without password."
  sudo -n "$XRAY_PATH" version
}

main() {
  case "${1:-}" in
    build)       build_all ;;
    start)       start_debug ;;
    stop)        stop_debug ;;
    restart)     stop_debug >/dev/null 2>&1 || true; start_debug ;;
    status)      show_status ;;
    check)       check ;;
    logs)        show_logs ;;
    sudo-setup)  setup_sudo ;;
    *)           usage; exit 1 ;;
  esac
}

main "$@"
