# Project Instructions

This repository is used as a Codex-managed recovery fork for the live
Picoclaw deployment on OrangePi plus its adjacent VPS control stack.

## Live Deployment Facts

- OrangePi gateway runtime: `picoclaw gateway`
- Public OrangePi hub: `http://100.101.105.50:3000/`
- Public control plane: `http://100.101.105.50:3000/dash/control`
- Legacy dashboard: `http://100.101.105.50:3000/legacy-dashboard`
- `/pico` is reserved for Pico transport/webhook traffic
- `picoclaw agent` is an interactive CLI, not a daemon

## Auth / Integrations

- Google Workspace credentials are already synced in the live runtime at
  `/root/.config/gws/credentials.json`.
- Local Codex host credentials also exist at
  `/home/caps/.config/gws/credentials.json`.
- Treat Drive/Calendar as intended integrations when working on the control
  plane, docs, and operator workflows.

## Skills

- Project-local skills live under `workspace/skills/`.
- Do not add project-local skills whose names conflict with system skills
  already provided to Codex.
- The repo-local duplicate `skill-creator` skill was removed for that reason;
  use the system `skill-creator` instead.

## Git Layout

- Expected fork target: `Capslockb/picoclaw`
- Upstream source: `sipeed/picoclaw`
- Keep deployment docs/config samples in git when they reflect real runtime
  state.
