#!/usr/bin/env python3
import argparse
import json
import urllib.error
import urllib.parse
import urllib.request

from common import getenv

ENV_FILE = "/etc/picoclaw/phase1.env"


def send_telegram(message: str, disable_notification: bool = False) -> tuple[bool, str]:
    token = getenv("TELEGRAM_BOT_TOKEN", env_file=ENV_FILE)
    chat_id = getenv("TELEGRAM_CHAT_ID", env_file=ENV_FILE)
    if not token or not chat_id:
        return False, "telegram_not_configured"

    url = f"https://api.telegram.org/bot{token}/sendMessage"
    payload = urllib.parse.urlencode(
        {
            "chat_id": chat_id,
            "text": message,
            "disable_notification": "true" if disable_notification else "false",
        }
    ).encode("utf-8")

    req = urllib.request.Request(url, data=payload, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            data = json.loads(body)
            if data.get("ok"):
                return True, "sent"
            return False, f"telegram_api_error:{data.get('description', 'unknown')}"
    except urllib.error.URLError as err:
        return False, f"network_error:{err}"


def main() -> int:
    parser = argparse.ArgumentParser(description="Send phase1 alert to Telegram")
    parser.add_argument("--message", required=True)
    parser.add_argument("--silent", action="store_true")
    args = parser.parse_args()

    ok, reason = send_telegram(args.message, disable_notification=args.silent)
    print(reason)
    return 0 if ok else 2


if __name__ == "__main__":
    raise SystemExit(main())
