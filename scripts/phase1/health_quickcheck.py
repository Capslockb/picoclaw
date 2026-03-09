#!/usr/bin/env python3
import json
import shutil
import urllib.error
import urllib.request

from checkpoint_store import CheckpointStore
from common import append_jsonl, ensure_dir, getenv, run_cmd, utc_now_iso
from telegram_alert_router import send_telegram

ENV_FILE = "/etc/picoclaw/phase1.env"
LOG_FILE = "/var/log/picoclaw/phase1/quickcheck.jsonl"
STATE_DIR = "/var/lib/picoclaw/phase1"


def http_probe(url: str, timeout_sec: int = 5) -> dict:
    try:
        req = urllib.request.Request(url, method="GET")
        with urllib.request.urlopen(req, timeout=timeout_sec) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            return {
                "ok": 200 <= resp.getcode() < 300,
                "status_code": resp.getcode(),
                "body": body[:800],
            }
    except urllib.error.URLError as err:
        return {"ok": False, "error": str(err)}


def main() -> int:
    ensure_dir(STATE_DIR)
    ensure_dir("/var/log/picoclaw/phase1")

    gateway = getenv("PICOCLAW_GATEWAY", "http://127.0.0.1:3000", env_file=ENV_FILE).rstrip("/")
    unit = getenv("PICOCLAW_UNIT", "picoclaw.service", env_file=ENV_FILE)
    disk_warn_pct = int(getenv("DISK_WARN_PERCENT", "90", env_file=ENV_FILE))

    store = CheckpointStore()
    store.set("runs.quickcheck_last_attempt", utc_now_iso())
    store.save()

    health = http_probe(f"{gateway}/health")
    ready = http_probe(f"{gateway}/ready")

    svc = run_cmd(["systemctl", "is-active", unit])
    service_active = svc.code == 0 and svc.stdout.strip() == "active"

    usage = shutil.disk_usage("/")
    used_pct = int((usage.used / usage.total) * 100) if usage.total > 0 else 0

    summary = {
        "timestamp": utc_now_iso(),
        "gateway": gateway,
        "service_unit": unit,
        "health": health,
        "ready": ready,
        "service_active": service_active,
        "service_stdout": svc.stdout,
        "service_stderr": svc.stderr,
        "disk_used_percent": used_pct,
        "disk_warn_percent": disk_warn_pct,
        "ok": bool(health.get("ok") and ready.get("ok") and service_active and used_pct < disk_warn_pct),
    }

    if summary["ok"]:
        store.set("runs.quickcheck_last_success", summary["timestamp"])
    store.save()

    append_jsonl(LOG_FILE, summary)

    if not summary["ok"]:
        lines = [
            "Phase1 quickcheck alert",
            f"service({unit}): {'ok' if service_active else 'failed'}",
            f"health: {'ok' if health.get('ok') else 'failed'}",
            f"ready: {'ok' if ready.get('ok') else 'failed'}",
            f"disk: {used_pct}%",
            f"host: {run_cmd(['hostname']).stdout}",
        ]
        send_telegram("\n".join(lines))

    print(json.dumps(summary, indent=2))
    return 0 if summary["ok"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
