#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

cmd="${1:-}"
case "$cmd" in
  quickcheck)
    exec python3 "$SCRIPT_DIR/health_quickcheck.py"
    ;;
  recover)
    exec python3 "$SCRIPT_DIR/missed_update_recovery.py"
    ;;
  *)
    echo "usage: $0 {quickcheck|recover}" >&2
    exit 2
    ;;
esac
