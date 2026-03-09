#!/usr/bin/env python3
import argparse
from typing import Any

from common import read_json, utc_now_iso, write_json_atomic

DEFAULT_PATH = "/var/lib/picoclaw/phase1/checkpoints.json"


class CheckpointStore:
    def __init__(self, path: str = DEFAULT_PATH):
        self.path = path
        self.data = read_json(path, self._default())
        self._ensure_shape()

    @staticmethod
    def _default() -> dict[str, Any]:
        return {
            "version": 1,
            "updated_at": None,
            "sources": {
                "telegram": {
                    "last_update_id": None,
                    "last_checked_at": None,
                },
                "whatsapp": {
                    "last_message_id": None,
                    "last_checked_at": None,
                },
                "email": {
                    "last_message_id": None,
                    "last_checked_at": None,
                },
            },
            "runs": {
                "quickcheck_last_success": None,
                "recovery_last_success": None,
                "quickcheck_last_attempt": None,
                "recovery_last_attempt": None,
            },
        }

    def _ensure_shape(self) -> None:
        default = self._default()
        if not isinstance(self.data, dict):
            self.data = default
            return
        for k, v in default.items():
            if k not in self.data:
                self.data[k] = v
        for src, default_obj in default["sources"].items():
            self.data["sources"].setdefault(src, default_obj)
            for k, v in default_obj.items():
                self.data["sources"][src].setdefault(k, v)
        for k, v in default["runs"].items():
            self.data["runs"].setdefault(k, v)

    def save(self) -> None:
        self.data["updated_at"] = utc_now_iso()
        write_json_atomic(self.path, self.data)

    def get(self, dotted_key: str, default: Any = None) -> Any:
        node: Any = self.data
        for part in dotted_key.split("."):
            if not isinstance(node, dict) or part not in node:
                return default
            node = node[part]
        return node

    def set(self, dotted_key: str, value: Any) -> None:
        parts = dotted_key.split(".")
        node: Any = self.data
        for part in parts[:-1]:
            if part not in node or not isinstance(node[part], dict):
                node[part] = {}
            node = node[part]
        node[parts[-1]] = value


def main() -> int:
    parser = argparse.ArgumentParser(description="Phase1 checkpoint store utility")
    parser.add_argument("--path", default=DEFAULT_PATH)
    parser.add_argument("--get")
    parser.add_argument("--set", nargs=2, metavar=("KEY", "VALUE"))
    args = parser.parse_args()

    store = CheckpointStore(args.path)

    if args.get:
        value = store.get(args.get)
        print(value)
        return 0

    if args.set:
        key, value = args.set
        store.set(key, value)
        store.save()
        print(f"ok: {key}")
        return 0

    print(store.data)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
