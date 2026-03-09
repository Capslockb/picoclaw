#!/usr/bin/env python3
import json
import pathlib
import urllib.error
import urllib.parse
import urllib.request

from checkpoint_store import CheckpointStore
from common import append_jsonl, ensure_dir, getenv, utc_now_iso, write_json_atomic
from telegram_alert_router import send_telegram

ENV_FILE = "/etc/picoclaw/phase1.env"
LOG_FILE = "/var/log/picoclaw/phase1/recovery.jsonl"
STATE_DIR = "/var/lib/picoclaw/phase1"


def fetch_telegram_updates(token: str, offset: int | None, limit: int = 100, timeout_sec: int = 15) -> dict:
    params = {
        "timeout": "0",
        "limit": str(limit),
    }
    if offset is not None:
        params["offset"] = str(offset)

    qs = urllib.parse.urlencode(params)
    url = f"https://api.telegram.org/bot{token}/getUpdates?{qs}"
    req = urllib.request.Request(url, method="GET")
    with urllib.request.urlopen(req, timeout=timeout_sec) as resp:
        body = resp.read().decode("utf-8", errors="replace")
        return json.loads(body)


def summarize_updates(updates: list[dict]) -> dict:
    chats = {}
    for upd in updates:
        msg = upd.get("message") or upd.get("edited_message") or {}
        chat = msg.get("chat") or {}
        key = str(chat.get("id", "unknown"))
        chats[key] = chats.get(key, 0) + 1
    return {"total": len(updates), "by_chat": chats}


def main() -> int:
    ensure_dir(STATE_DIR)
    ensure_dir("/var/log/picoclaw/phase1")

    store = CheckpointStore()
    store.set("runs.recovery_last_attempt", utc_now_iso())
    store.save()

    token = getenv("TELEGRAM_BOT_TOKEN", env_file=ENV_FILE)
    if not token:
        payload = {
            "timestamp": utc_now_iso(),
            "ok": False,
            "reason": "telegram_not_configured",
            "recovered": 0,
        }
        append_jsonl(LOG_FILE, payload)
        print(json.dumps(payload, indent=2))
        return 0

    last_update_id = store.get("sources.telegram.last_update_id")
    offset = int(last_update_id) + 1 if isinstance(last_update_id, int) else None

    try:
        data = fetch_telegram_updates(token, offset)
    except urllib.error.URLError as err:
        payload = {
            "timestamp": utc_now_iso(),
            "ok": False,
            "reason": f"network_error:{err}",
            "recovered": 0,
        }
        append_jsonl(LOG_FILE, payload)
        print(json.dumps(payload, indent=2))
        return 1

    if not data.get("ok"):
        payload = {
            "timestamp": utc_now_iso(),
            "ok": False,
            "reason": data.get("description", "telegram_api_error"),
            "recovered": 0,
        }
        append_jsonl(LOG_FILE, payload)
        print(json.dumps(payload, indent=2))
        return 1

    updates = data.get("result", [])
    summary = summarize_updates(updates)

    max_update_id = last_update_id
    if updates:
        max_update_id = max(upd.get("update_id", 0) for upd in updates)
        if isinstance(max_update_id, int) and max_update_id > 0:
            store.set("sources.telegram.last_update_id", max_update_id)

    now = utc_now_iso()
    store.set("sources.telegram.last_checked_at", now)
    store.set("runs.recovery_last_success", now)
    store.save()

    payload = {
        "timestamp": now,
        "ok": True,
        "recovered": summary["total"],
        "summary": summary,
        "last_update_id": max_update_id,
    }

    append_jsonl(LOG_FILE, payload)

    if summary["total"] > 0:
        ts = now.replace(":", "-")
        batch_path = pathlib.Path(STATE_DIR) / "recovered" / f"telegram-{ts}.json"
        write_json_atomic(str(batch_path), {"timestamp": now, "updates": updates, "summary": summary})

        lines = [
            "Phase1 recovered missed Telegram updates",
            f"count: {summary['total']}",
            f"by_chat: {json.dumps(summary['by_chat'], ensure_ascii=True)}",
        ]
        send_telegram("\n".join(lines), disable_notification=True)

    print(json.dumps(payload, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
