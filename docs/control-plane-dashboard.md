# Control Plane Dashboard

The gateway root route (`/`) can serve a multi-page operations console when the
gateway binary was built from a revision that includes
`pkg/channels/control_plane_dashboard.go`.

## Pages

1. Overview
2. Agents
3. Nodes
4. Tailscale / SSH
5. Auth / Setup
6. Jobs
7. Backend Test Chat
8. Media Workflows
9. App Generator
10. Logs / Artifacts

Legacy lightweight page is preserved at `/legacy-dashboard`.

`/pico` and `/pico/` are reserved for the Pico channel transport. Do not use
them as dashboard routes.

## API Endpoints

- `GET /api/control-plane/status`
- `GET /api/control-plane/test-chat/history`
- `POST /api/control-plane/test-chat`
- `POST /api/control-plane/action`
- `GET /api/control-plane/logs?source=heartbeat|dashboard&search=<text>`

## Compact Status Schema

`/api/control-plane/status` returns:

- `generated_at`, `gateway`
- `summary` (`nodes`, `agents`, `jobs_total`, `pending_setup`, `degraded_services`)
- `services[]` (`id`, `name`, `status`, `uptime`, `last_error`, `last_success`)
- `agents[]` (`id`, `type`, `state`, `active_job`, `queue_depth`, `capabilities`, `assigned_node`, ...)
- `nodes[]` (`id`, `hostname`, `role`, `tailscale_ip`, `ssh_status`, `reachability`, `exposed_services`, ...)
- `tailscale.paths[]` (node-to-node matrix + blocker reason)
- `auth.checklist[]`, `auth.providers[]`
- `jobs` (counters + `items[]` with timeline/artifacts)
- `media` (pipeline + templates)
- `app_generator` (requests + status)
- `logs.artifacts[]`
- `pending_setup[]`

## Cluster Wiring / Data Sources

Current adapters pull from:

- in-process channel manager state
- gateway-configured channels and webhook handlers
- auth store (`pkg/auth` credentials)
- workspace files (`heartbeat.log`, `sessions/`, artifacts)
- tailscale status (`tailscale status --json`)
- SSH readiness probes (`ssh -o BatchMode=yes ...` with caching)
- in-memory control-plane job/test-chat state

## Environment Variables

- `PICOCLAW_DASHBOARD_ALLOW_SHELL=true`
  - Enables shell command execution for `codex_execution` test type.
  - Default is disabled.
- Google Workspace auth is inferred from `~/.config/gws/credentials.json`.
  On the recovered OrangePi runtime, that file is present and Drive/Calendar
  readiness should report as authenticated unless credentials drift.
- `REDIS_URL`
  - Used for Redis service card readiness.
- `WORKER_API_URLS`
  - Used for worker API service card readiness.

## Startup / Dev

Run gateway normally; dashboard is served by the shared HTTP server:

```bash
picoclaw gateway
```

Route changes are compiled into the gateway binary. After editing dashboard code,
rebuild/install `picoclaw` and restart the gateway service before expecting new
routes to appear.

## Deployment Topologies

### Direct Gateway

With the repo defaults, the gateway listens on its configured port directly
(commonly `127.0.0.1:18790` unless you changed `gateway.port` in config).

- `http://127.0.0.1:18790/`
- `http://127.0.0.1:18790/legacy-dashboard`

### Reverse Proxy / Hub

Some deployments front the gateway with nginx on `:3000` and expose extra
dashboards under `/dash/...`. In that topology:

- `http://<host>:3000/`
- `http://<host>:3000/legacy-dashboard`

If you also proxy external dashboards, use a dedicated namespace such as:

- `/dash/system`
- `/dash/nas`
- `/dash/control`

Keep the gateway dashboard on `/` or `/legacy-dashboard`, not `/pico`.

## Node Registration Notes

Default node slots shown in UI:

- `pico` (self)
- `vps`
- `pc`
- `storage`

Discovery uses Tailscale hostnames/tags plus alias probing. For stricter mappings,
update `buildNodesAndTailscale()` and `rawSSHProbe()` in
`pkg/channels/control_plane_dashboard.go`.
