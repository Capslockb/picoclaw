#!/usr/bin/env python3
import json
import os
import pathlib
import subprocess
import tempfile
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, Optional


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat()


def load_env_file(path: str) -> Dict[str, str]:
    env: Dict[str, str] = {}
    p = pathlib.Path(path)
    if not p.exists():
        return env

    for raw in p.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        env[key.strip()] = value.strip().strip('"').strip("'")
    return env


def getenv(name: str, default: Optional[str] = None, env_file: Optional[str] = None) -> Optional[str]:
    value = os.getenv(name)
    if value is not None and value != "":
        return value
    if env_file:
        file_env = load_env_file(env_file)
        value = file_env.get(name)
        if value is not None and value != "":
            return value
    return default


def ensure_dir(path: str) -> None:
    pathlib.Path(path).mkdir(parents=True, exist_ok=True)


def read_json(path: str, default: Any) -> Any:
    p = pathlib.Path(path)
    if not p.exists():
        return default
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except Exception:
        return default


def write_json_atomic(path: str, data: Any) -> None:
    p = pathlib.Path(path)
    ensure_dir(str(p.parent))
    tmp_fd, tmp_name = tempfile.mkstemp(prefix=p.name + ".tmp.", dir=str(p.parent))
    try:
        with os.fdopen(tmp_fd, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2, sort_keys=True)
            f.write("\n")
        os.replace(tmp_name, path)
    finally:
        if os.path.exists(tmp_name):
            os.unlink(tmp_name)


def append_jsonl(path: str, payload: Dict[str, Any]) -> None:
    p = pathlib.Path(path)
    ensure_dir(str(p.parent))
    with p.open("a", encoding="utf-8") as f:
        f.write(json.dumps(payload, ensure_ascii=True) + "\n")


@dataclass
class CmdResult:
    code: int
    stdout: str
    stderr: str


def run_cmd(args: list[str], timeout_sec: int = 15) -> CmdResult:
    proc = subprocess.run(args, text=True, capture_output=True, timeout=timeout_sec, check=False)
    return CmdResult(code=proc.returncode, stdout=proc.stdout.strip(), stderr=proc.stderr.strip())
