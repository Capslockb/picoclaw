# Recovery Audit 2026-03-09

## Confirmed Live Orange Pi Topology

- Public hub: nginx on `:3000`
- Gateway binary: `picoclaw gateway` on `127.0.0.1:3001`
- Python dashboard: `/dash/system` proxied to `127.0.0.1:5000`
- Python dashboard: `/dash/nas` proxied to `127.0.0.1:8088`

## Live URLs

- `http://100.101.105.50:3000/`
  - nginx hub page
- `http://100.101.105.50:3000/dash/control`
  - restored control-plane dashboard from the gateway
- `http://100.101.105.50:3000/health`
  - live gateway health
- `http://100.101.105.50:3000/ready`
  - nginx readiness stub
- `http://100.101.105.50:3000/pico`
  - Pico webhook info endpoint
- `http://100.101.105.50:3000/dash/system`
  - Orange Pi system dashboard
- `http://100.101.105.50:3000/dash/nas`
  - NAS dashboard

## Important Findings

1. `/pico` is not a dashboard route.
   It is reserved for the Pico channel transport and should stay that way.

2. The repo contains a newer control-plane dashboard implementation in
   `pkg/channels/control_plane_dashboard.go`. During recovery, the live
   gateway binary was rebuilt and reinstalled so the documented
   `/api/control-plane/*` routes are now active again.

3. `picoclaw-worker.service` was invalid.
   It launched `picoclaw agent`, which is an interactive CLI rather than a
   long-running daemon. The unit was disabled on the Orange Pi to stop the
   restart loop.

4. The repo had deployment-cohesion problems:
   - stale `model_name` examples
   - leaked Telegram values in `systemd/phase1/phase1.env.example`
   - missing main gateway systemd sample
   - dashboard docs that did not explain direct vs proxied deployments
   - template workspace instructions that looked like live runtime state
   - unsupported Telegram formatting regex in `pkg/channels/telegram/telegram.go`
     that crashed staged builds until it was replaced with the safe parser now
     present in the recovery repo

5. Google Workspace auth was already present on the OrangePi runtime.
   `~/.config/gws/credentials.json` exists, so Drive/Calendar readiness in the
   control plane should be treated as an active integration rather than a
   placeholder.

## VPS Snapshot

- Host: `187.77.75.173`
- Main process family: `ironclaw`
- Open ports observed:
  - `3000`
  - `8080`
  - `50051`
  - `80` and `443` via `traefik`
- Docker is active
- Relevant files present:
  - `/root/ironclaw`
  - `/root/openclaw-workspace-template`
  - `/srv/docker/openclaw-kkns`

The VPS is reachable with password auth, but it is not yet ready for unattended
Codex automation because passwordless SSH was not configured from this machine.

## Recommended Next Steps

1. Keep `/pico` reserved for transport/webhook traffic.
2. If the VPS is meant to host public control surfaces, add a repeatable SSH
   path and document which of `ironclaw`, `openclaw`, or `picoclaw` owns which
   public ports.
3. Keep versioned deployment artifacts in git:
   - `systemd/picoclaw.service`
   - `systemd/nginx/picoclaw-3000.conf`
   - reverse-proxy sample config
   - deployment topology doc
