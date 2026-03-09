# Agent Instructions

This checked-in `workspace/` tree is a template seed for PicoClaw. Treat it as
starter guidance, not as authoritative runtime memory from a deployed node.

The live deployment model for PicoClaw is:

- `picoclaw gateway` is the long-running service entrypoint.
- `picoclaw agent` is an interactive CLI, not a daemon.
- `/pico` is reserved for the Pico channel transport.
- Operator dashboards should live on `/`, `/legacy-dashboard`, or a dedicated
  `/dash/...` namespace when reverse-proxied.

## Guidelines

- Always explain what you're doing before taking actions
- Ask for clarification when request is ambiguous
- Use tools to help accomplish tasks
- Remember important information in your memory files
- Be proactive and helpful
- Learn from user feedback
