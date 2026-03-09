# Phase1 (Orange Pi SSH)

Phase1 initializes the first autonomous maintenance slice:

1. `health_quickcheck.py`
   - checks `PICOCLAW_GATEWAY/health` and `/ready`
   - checks `picoclaw.service` active state
   - checks root disk pressure
   - logs to `/var/log/picoclaw/phase1/quickcheck.jsonl`
2. `missed_update_recovery.py`
   - checkpointed Telegram `getUpdates` recovery
   - updates `/var/lib/picoclaw/phase1/checkpoints.json`
   - stores recovered batches under `/var/lib/picoclaw/phase1/recovered/`
   - logs to `/var/log/picoclaw/phase1/recovery.jsonl`
3. `telegram_alert_router.py`
   - sends alert summaries to your Telegram chat when configured

## Install / Initialize

```bash
cd /root/picoclaw
./scripts/phase1/init_phase1.sh
```

## Configure

Edit:

```bash
sudo nano /etc/picoclaw/phase1.env
```

Set at minimum:

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

## Manual run

```bash
cd /root/picoclaw
./scripts/phase1/run_phase1.sh quickcheck
./scripts/phase1/run_phase1.sh recover
```

## systemd units

- `picoclaw-phase1-quickcheck.timer` (startup + hourly)
- `picoclaw-phase1-recovery.timer` (every 15 minutes)

## State paths

- checkpoints: `/var/lib/picoclaw/phase1/checkpoints.json`
- recovery batches: `/var/lib/picoclaw/phase1/recovered/*.json`
- quickcheck logs: `/var/log/picoclaw/phase1/quickcheck.jsonl`
- recovery logs: `/var/log/picoclaw/phase1/recovery.jsonl`
