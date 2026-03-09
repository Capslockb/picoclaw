#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
SYSTEMD_SRC="$ROOT_DIR/systemd/phase1"
ENV_TARGET_DIR="/etc/picoclaw"
ENV_TARGET="$ENV_TARGET_DIR/phase1.env"

sudo mkdir -p "$ENV_TARGET_DIR" /var/lib/picoclaw/phase1 /var/log/picoclaw/phase1

if [[ ! -f "$ENV_TARGET" ]]; then
  sudo cp "$SYSTEMD_SRC/phase1.env.example" "$ENV_TARGET"
  echo "created: $ENV_TARGET"
  echo "edit TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID before relying on Telegram recovery/alerts"
else
  echo "exists: $ENV_TARGET"
fi

sudo cp "$SYSTEMD_SRC/picoclaw-phase1-quickcheck.service" /etc/systemd/system/
sudo cp "$SYSTEMD_SRC/picoclaw-phase1-quickcheck.timer" /etc/systemd/system/
sudo cp "$SYSTEMD_SRC/picoclaw-phase1-recovery.service" /etc/systemd/system/
sudo cp "$SYSTEMD_SRC/picoclaw-phase1-recovery.timer" /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now picoclaw-phase1-quickcheck.timer
sudo systemctl enable --now picoclaw-phase1-recovery.timer

# Run once immediately for initialization.
sudo systemctl start picoclaw-phase1-quickcheck.service || true
sudo systemctl start picoclaw-phase1-recovery.service || true

echo
sudo systemctl --no-pager --full status picoclaw-phase1-quickcheck.timer | sed -n '1,12p'
echo
sudo systemctl --no-pager --full status picoclaw-phase1-recovery.timer | sed -n '1,12p'
