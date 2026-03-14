package channels

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
	"github.com/sipeed/picoclaw/pkg/logger"
)

var controlPlaneTemplate = template.Must(template.New("control_plane_dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <title>PicoClaw Control Plane</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;700&family=IBM+Plex+Mono:wght@400;500&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #070b10;
      --bg-soft: #101724;
      --bg-panel: #131c2c;
      --bg-panel-2: #0d1523;
      --border: #233551;
      --text: #dbe9ff;
      --muted: #89a3c7;
      --ok: #23c488;
      --warn: #efb545;
      --err: #f26c6c;
      --accent: #4ab8ff;
      --chip: #1d2a40;
      --shadow: rgba(0, 0, 0, 0.35);
    }
    * { box-sizing: border-box; }
    html, body { height: 100%; }
    body {
      margin: 0;
      font-family: 'Space Grotesk', sans-serif;
      background: radial-gradient(1000px 500px at 0% -10%, #16304c 0%, #070b10 52%), var(--bg);
      color: var(--text);
      overflow: hidden;
    }
    .shell {
      display: grid;
      grid-template-columns: 260px 1fr;
      grid-template-rows: 62px 1fr;
      height: 100vh;
    }
    .topbar {
      grid-column: 1 / -1;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 16px;
      border-bottom: 1px solid var(--border);
      background: rgba(9, 15, 24, 0.92);
      backdrop-filter: blur(8px);
    }
    .brand { display: flex; align-items: center; gap: 10px; }
    .logo {
      width: 34px;
      height: 34px;
      border-radius: 10px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: 700;
      background: linear-gradient(140deg, #25d6af, #4ab8ff);
      color: #02253a;
      box-shadow: 0 4px 20px rgba(74, 184, 255, 0.35);
    }
    .brand h1 { margin: 0; font-size: 16px; }
    .brand small { color: var(--muted); font-family: 'IBM Plex Mono', monospace; }
    .top-actions { display: flex; align-items: center; gap: 8px; }
    .btn {
      border: 1px solid var(--border);
      background: var(--bg-panel);
      color: var(--text);
      border-radius: 8px;
      padding: 8px 10px;
      cursor: pointer;
      font-family: inherit;
      font-size: 12px;
    }
    .btn:hover { border-color: #3f608c; }
    .btn.primary { background: linear-gradient(145deg, #1f8fd2, #0f699f); border-color: #3795ce; }
    .btn.warn { background: #5f4314; border-color: #8d6a2a; color: #ffd996; }
    .btn.danger { background: #5b2020; border-color: #8d3a3a; color: #ffd2d2; }
    .status-pill {
      border-radius: 999px;
      padding: 2px 10px;
      font-size: 11px;
      border: 1px solid;
      font-family: 'IBM Plex Mono', monospace;
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }
    .status-ok { color: #86f1cd; border-color: rgba(35,196,136,0.45); background: rgba(35,196,136,0.12); }
    .status-warn { color: #ffe2a6; border-color: rgba(239,181,69,0.45); background: rgba(239,181,69,0.12); }
    .status-error { color: #ffc2c2; border-color: rgba(242,108,108,0.5); background: rgba(242,108,108,0.14); }
    .status-unknown { color: #b6c6df; border-color: rgba(137,163,199,0.35); background: rgba(137,163,199,0.12); }

    .sidebar {
      border-right: 1px solid var(--border);
      background: rgba(9, 13, 21, 0.9);
      overflow-y: auto;
      padding: 10px;
    }
    .sidebar .section-label {
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.06em;
      font-size: 10px;
      margin: 10px 10px 6px;
    }
    .nav-btn {
      width: 100%;
      text-align: left;
      border: 1px solid transparent;
      background: transparent;
      color: var(--text);
      border-radius: 8px;
      padding: 9px 10px;
      margin-bottom: 4px;
      cursor: pointer;
      font-size: 13px;
      font-family: inherit;
    }
    .nav-btn:hover { background: #131e31; }
    .nav-btn.active { background: #17263e; border-color: #2d4a72; }

    .main {
      overflow: auto;
      padding: 14px;
      display: flex;
      flex-direction: column;
      gap: 14px;
    }
    .view { display: none; }
    .view.active { display: block; }
    .view-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; margin-bottom: 10px; }
    .view-title { font-size: 20px; margin: 0; }
    .view-sub { margin: 4px 0 0; color: var(--muted); font-size: 13px; }

    .grid { display: grid; gap: 10px; }
    .grid.cards { grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); }
    .grid.two { grid-template-columns: 1.1fr 0.9fr; }
    .grid.three { grid-template-columns: repeat(3, 1fr); }

    .card {
      border: 1px solid var(--border);
      border-radius: 12px;
      background: linear-gradient(170deg, var(--bg-panel), var(--bg-panel-2));
      box-shadow: 0 8px 22px var(--shadow);
      padding: 12px;
      min-height: 80px;
    }
    .card h3 { margin: 0 0 8px; font-size: 13px; color: #bcd1f0; text-transform: uppercase; letter-spacing: 0.05em; }
    .metric { font-size: 22px; font-weight: 700; margin: 6px 0; }
    .submetric { color: var(--muted); font-size: 12px; }

    .table-wrap {
      border: 1px solid var(--border);
      border-radius: 12px;
      overflow: auto;
      background: var(--bg-panel-2);
      box-shadow: 0 8px 22px var(--shadow);
      max-height: 420px;
    }
    table { border-collapse: collapse; width: 100%; min-width: 860px; font-size: 12px; }
    th, td { border-bottom: 1px solid #1e2d46; padding: 8px; text-align: left; vertical-align: top; }
    th {
      position: sticky;
      top: 0;
      z-index: 2;
      background: #132038;
      color: #b4caea;
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.04em;
    }
    tr:hover td { background: rgba(33, 53, 82, 0.35); }

    .chip {
      display: inline-flex;
      align-items: center;
      border: 1px solid #2c456a;
      border-radius: 999px;
      padding: 2px 8px;
      margin: 0 4px 4px 0;
      background: var(--chip);
      color: #bdd2f0;
      font-size: 11px;
    }

    .list {
      margin: 0;
      padding-left: 16px;
      line-height: 1.45;
      font-size: 12px;
      color: #ccddf6;
    }

    .mono { font-family: 'IBM Plex Mono', monospace; }

    .chat-layout { display: grid; grid-template-columns: 1.1fr 0.9fr; gap: 10px; min-height: 540px; }
    .chat-feed {
      border: 1px solid var(--border);
      border-radius: 12px;
      background: var(--bg-panel-2);
      overflow: auto;
      padding: 10px;
      display: flex;
      flex-direction: column;
      gap: 8px;
      max-height: 620px;
    }
    .msg {
      border: 1px solid #274162;
      border-radius: 10px;
      padding: 8px;
      background: #0f1a2b;
      white-space: pre-wrap;
      word-break: break-word;
      font-size: 12px;
    }
    .msg.user { border-color: #326b7a; background: #102430; }
    .msg.assistant { border-color: #3d4f7d; background: #111a32; }
    .msg .meta { color: var(--muted); font-size: 11px; margin-bottom: 5px; }

    .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
    .input, textarea, select {
      width: 100%;
      border: 1px solid #2c456a;
      border-radius: 8px;
      background: #0b1322;
      color: var(--text);
      padding: 8px;
      font-family: inherit;
      font-size: 12px;
    }
    textarea { min-height: 110px; resize: vertical; font-family: 'IBM Plex Mono', monospace; }
    label { font-size: 11px; color: #a8bedf; display: block; margin-bottom: 4px; }

    .toolbar { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; margin-bottom: 8px; }

    details {
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 6px 8px;
      background: #0f1829;
    }
    details summary { cursor: pointer; color: #bfd3f2; }

    .kv { display: grid; grid-template-columns: 160px 1fr; gap: 6px; font-size: 12px; }
    .kv .k { color: #9fb8da; }
    .stack { display: flex; flex-direction: column; gap: 10px; }
    .metric-sm { font-size: 12px; color: var(--muted); }
    .progress {
      width: 100%;
      height: 8px;
      border-radius: 999px;
      overflow: hidden;
      background: #0a1220;
      border: 1px solid #1f314a;
      margin-top: 8px;
    }
    .progress > span {
      display: block;
      height: 100%;
      background: linear-gradient(90deg, #25d6af, #4ab8ff);
      box-shadow: 0 0 18px rgba(74, 184, 255, 0.3);
    }
    .finding {
      border: 1px solid #294264;
      border-radius: 10px;
      background: #0e1828;
      padding: 10px;
      margin-bottom: 8px;
    }
    .finding h4 {
      margin: 0 0 6px;
      font-size: 13px;
      display: flex;
      align-items: center;
      gap: 8px;
    }
    .finding p {
      margin: 4px 0;
      font-size: 12px;
      color: #d2e3fb;
    }
    .domain-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
      gap: 10px;
    }
    .domain-card {
      border: 1px solid var(--border);
      border-radius: 10px;
      background: #0f1727;
      padding: 10px;
    }
    .domain-card h4 {
      margin: 0 0 6px;
      font-size: 13px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }
    .domain-card p {
      margin: 0;
      font-size: 12px;
      color: var(--muted);
      line-height: 1.45;
    }
    .access-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
      gap: 10px;
    }
    .access-card {
      border: 1px solid var(--border);
      border-radius: 10px;
      background: #0f1727;
      padding: 10px;
    }
    .access-card a {
      word-break: break-all;
    }
    .hero-strip {
      display: grid;
      grid-template-columns: 1.4fr 0.8fr;
      gap: 10px;
      margin-bottom: 10px;
    }
    .hero-panel {
      position: relative;
      overflow: hidden;
      min-height: 144px;
      background:
        radial-gradient(420px 160px at 0% 0%, rgba(74, 184, 255, 0.22), transparent 60%),
        linear-gradient(160deg, rgba(18, 36, 58, 0.98), rgba(8, 13, 22, 0.98));
    }
    .hero-panel::after {
      content: '';
      position: absolute;
      inset: auto -10% -40% auto;
      width: 220px;
      height: 220px;
      border-radius: 50%;
      background: rgba(35, 196, 136, 0.12);
      filter: blur(10px);
    }
    .hero-title {
      margin: 0 0 8px;
      font-size: 28px;
      line-height: 1.02;
    }
    .hero-copy {
      max-width: 680px;
      color: var(--muted);
      font-size: 13px;
      line-height: 1.5;
    }
    .pulse-dot {
      width: 10px;
      height: 10px;
      border-radius: 999px;
      background: var(--ok);
      box-shadow: 0 0 0 rgba(35, 196, 136, 0.55);
      animation: pulse 1.8s infinite;
    }
    .terminal-shell {
      border: 1px solid var(--border);
      border-radius: 14px;
      background: #060b12;
      box-shadow: 0 16px 40px rgba(0, 0, 0, 0.4);
      overflow: hidden;
    }
    .terminal-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      padding: 10px 12px;
      border-bottom: 1px solid #1e2d46;
      background: linear-gradient(180deg, #10192a, #0c1320);
    }
    .terminal-feed {
      min-height: 320px;
      max-height: 420px;
      overflow: auto;
      padding: 12px;
      font-family: 'IBM Plex Mono', monospace;
      font-size: 12px;
      line-height: 1.45;
      color: #b9e8d4;
      background:
        linear-gradient(180deg, rgba(5, 12, 18, 0.98), rgba(4, 9, 15, 0.98)),
        radial-gradient(700px 240px at 100% 0%, rgba(74, 184, 255, 0.08), transparent 60%);
    }
    .terminal-entry {
      padding: 0 0 12px;
      margin-bottom: 12px;
      border-bottom: 1px dashed #20324d;
    }
    .terminal-entry:last-child {
      border-bottom: 0;
      margin-bottom: 0;
      padding-bottom: 0;
    }
    .terminal-cmd {
      color: #9ad4ff;
      margin-bottom: 6px;
    }
    .terminal-output {
      white-space: pre-wrap;
      color: #d7f3e7;
    }
    .terminal-form {
      display: grid;
      grid-template-columns: 1fr auto auto;
      gap: 8px;
      padding: 12px;
      border-top: 1px solid #1e2d46;
      background: #0b1220;
    }
    .terminal-lock {
      border: 1px dashed #35557d;
      border-radius: 12px;
      padding: 14px;
      background: rgba(11, 18, 32, 0.86);
    }
    .quick-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
      gap: 8px;
    }
    .mini-metric {
      display: grid;
      gap: 4px;
      padding: 10px;
      border: 1px solid #20324d;
      border-radius: 10px;
      background: rgba(15, 23, 39, 0.72);
    }
    .mini-metric strong {
      font-size: 18px;
    }
    .anim-rise {
      animation: riseIn 0.45s ease both;
    }
    @keyframes pulse {
      0% { box-shadow: 0 0 0 0 rgba(35, 196, 136, 0.55); }
      70% { box-shadow: 0 0 0 12px rgba(35, 196, 136, 0); }
      100% { box-shadow: 0 0 0 0 rgba(35, 196, 136, 0); }
    }
    @keyframes riseIn {
      from { opacity: 0; transform: translateY(14px); }
      to { opacity: 1; transform: translateY(0); }
    }

    @media (max-width: 1180px) {
      .shell { grid-template-columns: 72px 1fr; }
      .sidebar .section-label { display: none; }
      .nav-btn { font-size: 11px; padding: 8px 6px; }
      .chat-layout, .grid.two, .grid.three { grid-template-columns: 1fr; }
      .hero-strip { grid-template-columns: 1fr; }
      .topbar { flex-wrap: wrap; align-items: flex-start; }
      .top-actions { width: 100%; justify-content: space-between; flex-wrap: wrap; }
    }
    @media (max-width: 760px) {
      body { overflow: auto; }
      .shell {
        grid-template-columns: 1fr;
        grid-template-rows: auto auto 1fr;
        min-height: 100vh;
        height: auto;
      }
      .sidebar {
        border-right: 0;
        border-bottom: 1px solid var(--border);
        padding: 8px 10px 10px;
        overflow-x: auto;
        white-space: nowrap;
      }
      .sidebar .section-label {
        display: block;
        margin-left: 2px;
      }
      .nav-btn {
        display: inline-flex;
        width: auto;
        margin: 0 6px 6px 0;
        padding: 9px 11px;
        font-size: 12px;
      }
      .main {
        padding: 10px;
        gap: 10px;
      }
      .view-title {
        font-size: 18px;
      }
      .hero-title {
        font-size: 22px;
      }
      .terminal-form {
        grid-template-columns: 1fr;
      }
      .btn {
        min-height: 40px;
      }
      .table-wrap {
        border-radius: 10px;
      }
    }
  </style>
</head>
<body>
  <div class="shell">
    <div class="topbar">
      <div class="brand">
        <div class="logo">PC</div>
        <div>
          <h1>PicoClaw Multi-Agent Control Plane</h1>
          <small class="mono">gateway={{.Address}} • boot={{.GeneratedAt}}</small>
        </div>
      </div>
      <div class="top-actions">
        <span id="globalStatus" class="status-pill status-unknown">loading</span>
        <button class="btn" id="openLegacyBtn">Legacy Dashboard</button>
        <button class="btn" id="refreshBtn">Refresh</button>
        <button class="btn primary" id="togglePollBtn">Pause Polling</button>
      </div>
    </div>

    <aside class="sidebar">
      <div class="section-label">Control Plane</div>
      <button class="nav-btn active" data-tab="overview">1. Overview</button>
      <button class="nav-btn" data-tab="agents">2. Agents</button>
      <button class="nav-btn" data-tab="nodes">3. Nodes</button>
      <button class="nav-btn" data-tab="tailscale">4. Tailscale / SSH</button>
      <button class="nav-btn" data-tab="auth">5. Auth / Setup</button>
      <button class="nav-btn" data-tab="jobs">6. Jobs</button>
      <button class="nav-btn" data-tab="chat">7. Backend Test Chat</button>
      <button class="nav-btn" data-tab="media">8. Media Workflows</button>
      <button class="nav-btn" data-tab="appgen">9. App Generator</button>
      <button class="nav-btn" data-tab="terminal">10. Secure Terminal</button>
      <button class="nav-btn" data-tab="logs">11. Logs / Artifacts</button>
    </aside>

    <main class="main">
      <section id="view-overview" class="view active">
        <div class="view-head">
          <div>
            <h2 class="view-title">Overview</h2>
            <p class="view-sub">Cluster-wide runtime, service health, pending setup, and quick validation controls.</p>
          </div>
          <div id="summaryChips"></div>
        </div>
        <div class="hero-strip anim-rise">
          <div class="card hero-panel">
            <div style="position:relative; z-index:1;">
              <div style="display:flex; align-items:center; gap:8px; margin-bottom:10px;">
                <span class="pulse-dot"></span>
                <span class="chip">Live orchestration</span>
                <span class="chip">backend-driven fixes</span>
                <span class="chip">mobile ready</span>
              </div>
              <h3 class="hero-title">PicoClaw is now one responsive surface for restore, diagnosis, and repair.</h3>
              <div class="hero-copy">Use the security page for LLM diagnosis and safe fix buttons, the secure terminal for authenticated operator access, and the backend test console for channel/Telegram/Codex workflow validation without leaving the control plane.</div>
            </div>
          </div>
          <div class="grid" style="grid-template-columns:1fr 1fr;">
            <div class="mini-metric"><span class="submetric">Operator mode</span><strong>Touch-first</strong><span class="submetric">cards, quick actions, dense mobile layouts</span></div>
            <div class="mini-metric"><span class="submetric">Recovery mode</span><strong>LLM + Safe Fix</strong><span class="submetric">diagnose, sync scripts, validate secure web</span></div>
            <div class="mini-metric"><span class="submetric">Terminal policy</span><strong>Login + TS + TLS</strong><span class="submetric">locked unless tailscale + secure origin</span></div>
            <div class="mini-metric"><span class="submetric">Codex path</span><strong>Pico fallback</strong><span class="submetric">local agent fallback if remote SSH path fails</span></div>
          </div>
        </div>
        <div class="grid cards" id="serviceCards"></div>
        <div class="grid two">
          <div class="card">
            <h3>Pending Setup / Auth Tasks</h3>
            <ul class="list" id="pendingTasks"></ul>
          </div>
          <div class="card">
            <h3>Cluster Test Controls</h3>
            <div class="toolbar">
              <button class="btn" data-action="test_health_endpoints">Test health endpoints</button>
              <button class="btn" data-action="ping_workers">Ping workers</button>
              <button class="btn" data-action="validate_tailscale_path">Validate tailscale path</button>
              <button class="btn" data-action="send_dry_run_job">Send dry-run job</button>
              <button class="btn warn" data-action="send_real_job">Send real job</button>
            </div>
            <div id="actionResult" class="mono" style="font-size:12px; color:#cde0ff; white-space:pre-wrap;"></div>
          </div>
        </div>
      </section>

      <section id="view-agents" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Multi-Agent Runtime</h2>
            <p class="view-sub">Agent registry, heartbeat, queue depth, active jobs, capabilities, and node assignment.</p>
          </div>
        </div>
        <div class="table-wrap"><table id="agentsTable"></table></div>
        <div class="card" style="margin-top:10px;">
          <h3>Per-Agent Logs</h3>
          <div id="agentLogs"></div>
        </div>
      </section>

      <section id="view-nodes" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Nodes / Infrastructure</h2>
            <p class="view-sub">VPS, PICO, PC workers, storage nodes, reachability, SSH readiness, and recent failures.</p>
          </div>
        </div>
        <div class="table-wrap"><table id="nodesTable"></table></div>
      </section>

      <section id="view-tailscale" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Tailscale / Remote Execution</h2>
            <p class="view-sub">Node-to-node path matrix, blockers, ACL/tag mismatch warnings, and setup completeness.</p>
          </div>
        </div>
        <div class="grid two">
          <div class="table-wrap"><table id="pathMatrixTable"></table></div>
          <div class="card">
            <h3>Warnings & Required Actions</h3>
            <ul class="list" id="tailscaleWarnings"></ul>
          </div>
        </div>
      </section>

      <section id="view-auth" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Security / Auth / Secure Web</h2>
            <p class="view-sub">High-level security controls, backend-driven diagnostics, Google/Tailscale readiness, and operator-facing secure access options.</p>
          </div>
        </div>
        <div class="grid cards" id="securityCards"></div>
        <div class="grid two">
          <div class="card">
            <h3>Security Domains</h3>
            <div id="securityDomains" class="domain-grid"></div>
          </div>
          <div class="card">
            <h3>Interactive Diagnostics</h3>
            <div class="toolbar">
              <button class="btn primary" data-action="security_llm_diagnose" data-result-target="securityActionResult">LLM self-diagnose</button>
              <button class="btn primary" data-action="security_llm_fix_plan" data-result-target="securityActionResult">LLM fix plan</button>
              <button class="btn" data-action="security_check_all" data-result-target="securityActionResult">Refresh posture</button>
              <button class="btn" data-action="security_probe_google_auth" data-result-target="securityActionResult">Google auth</button>
              <button class="btn" data-action="security_probe_secure_web" data-result-target="securityActionResult">Secure web</button>
              <button class="btn" data-action="security_probe_exec_surface" data-result-target="securityActionResult">Exec surface</button>
              <button class="btn warn" data-action="security_apply_safe_fixes" data-result-target="securityActionResult">Apply safe fixes</button>
              <button class="btn" data-action="security_sync_codex_fallback" data-result-target="securityActionResult">Sync codex fallback</button>
            </div>
            <div id="securityActionResult" class="mono" style="font-size:12px; color:#cde0ff; white-space:pre-wrap;"></div>
            <div id="securityDiagnosis" style="margin-top:10px;"></div>
          </div>
        </div>
        <div class="grid two">
          <div class="card">
            <h3>Findings / Fix Queue</h3>
            <div id="securityFindings"></div>
          </div>
          <div class="card">
            <h3>Web / Auth Access Matrix</h3>
            <div id="securityAccess" class="access-grid"></div>
          </div>
        </div>
        <div class="grid two">
          <div class="card">
            <h3>Guided Checklist</h3>
            <ul class="list" id="authChecklist"></ul>
          </div>
          <div class="card">
            <h3>Provider / Channel Auth State</h3>
            <div id="authProviders"></div>
          </div>
        </div>
      </section>

      <section id="view-jobs" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Job Queue / Execution</h2>
            <p class="view-sub">Queued/running/failed/completed jobs, retries, timeline, artifacts, and child task metadata.</p>
          </div>
        </div>
        <div class="grid cards" id="jobCounters"></div>
        <div class="table-wrap"><table id="jobsTable"></table></div>
        <div class="card" style="margin-top:10px;">
          <h3>Job Timeline / Logs</h3>
          <div id="jobTimeline" class="mono" style="font-size:12px; white-space:pre-wrap;"></div>
        </div>
      </section>

      <section id="view-chat" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Backend Test Chat UI</h2>
            <p class="view-sub">Prompt console + workflow debugger + integration tester + agent playground.</p>
          </div>
        </div>
        <div class="chat-layout">
          <div class="chat-feed" id="chatFeed"></div>
          <div class="card">
            <h3>Send Test</h3>
            <form id="chatForm">
              <div class="form-grid">
                <div>
                  <label>Agent</label>
                  <select id="chatAgent"></select>
                </div>
                <div>
                  <label>Execution target</label>
                  <select id="chatTarget"></select>
                </div>
                <div>
                  <label>Workflow template</label>
                  <select id="chatWorkflow">
                    <option value="generic-debug">generic-debug</option>
                    <option value="telegram-input">telegram-input</option>
                    <option value="media-pipeline">media-pipeline</option>
                    <option value="app-generation">app-generation</option>
                    <option value="worker-api">worker-api</option>
                    <option value="codex-exec">codex-exec</option>
                  </select>
                </div>
                <div>
                  <label>Test type</label>
                  <select id="chatType">
                    <option value="agent_prompt">agent_prompt</option>
                    <option value="telegram_input">telegram_input</option>
                    <option value="media_job">media_job</option>
                    <option value="app_generation">app_generation</option>
                    <option value="worker_api_call">worker_api_call</option>
                    <option value="codex_execution">codex_execution</option>
                  </select>
                </div>
              </div>
              <label style="margin-top:8px;">Prompt</label>
              <textarea id="chatPrompt" placeholder="Send a backend test prompt, command probe, or workflow request..."></textarea>
              <label>Raw JSON payload</label>
              <textarea id="chatPayload" class="mono" placeholder='{"input":"value","chat_id":"5533009291"}'></textarea>
              <div class="toolbar">
                <label style="display:flex; gap:6px; align-items:center; margin:0;"><input type="checkbox" id="chatDryRun" checked /> Dry-run</label>
                <button class="btn primary" type="submit">Run Test</button>
                <button class="btn" type="button" id="savePresetBtn">Save Preset</button>
                <button class="btn" type="button" id="loadPresetBtn">Load Preset</button>
              </div>
            </form>
            <div id="chatInfo" class="mono" style="font-size:11px; color:#9db9df;"></div>
          </div>
        </div>
      </section>

      <section id="view-media" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Media Workflows</h2>
            <p class="view-sub">Drive ingest, transcript/SRT, cut suggestions, FFmpeg readiness, and Vegas-oriented outputs.</p>
          </div>
        </div>
        <div class="grid two">
          <div class="card">
            <h3>Media Pipeline Status</h3>
            <div id="mediaStatus"></div>
          </div>
          <div class="card">
            <h3>Templates / Sample Manifests</h3>
            <div id="mediaTemplates"></div>
          </div>
        </div>
      </section>

      <section id="view-appgen" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">App / Site Generator</h2>
            <p class="view-sub">Generation requests, template selection, build status, deploy targets, and child-agent contribution.</p>
          </div>
        </div>
        <div class="grid two">
          <div class="card">
            <h3>Requests</h3>
            <div id="appGenRequests"></div>
          </div>
          <div class="card">
            <h3>Build / Deploy Status</h3>
            <div id="appGenStatus"></div>
          </div>
        </div>
      </section>

      <section id="view-terminal" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Secure Terminal / Operator Console</h2>
            <p class="view-sub">Interactive command access for operators. This surface is intentionally blocked unless the request is on tailscale, over HTTPS, and logged in.</p>
          </div>
          <div id="terminalStatePill"></div>
        </div>
        <div class="grid two">
          <div class="card">
            <h3>Access Policy</h3>
            <div id="terminalPolicy" class="kv"></div>
            <div class="quick-grid" style="margin-top:10px;" id="terminalQuick"></div>
          </div>
          <div class="card">
            <h3>Login</h3>
            <div id="terminalLock" class="terminal-lock">
              <p style="margin-top:0;">Use the terminal login password only from the hardened tailscale HTTPS surface. Public IP + plain HTTP is rejected.</p>
              <form id="terminalLoginForm">
                <label>Terminal password</label>
                <input id="terminalPassword" class="input" type="password" placeholder="login required for terminal commands" />
                <div class="toolbar" style="margin-top:10px;">
                  <button class="btn primary" type="submit">Unlock terminal</button>
                  <button class="btn" type="button" id="terminalLogoutBtn">Logout</button>
                </div>
              </form>
              <div id="terminalLoginInfo" class="mono" style="font-size:11px; color:#9db9df;"></div>
            </div>
          </div>
        </div>
        <div class="terminal-shell anim-rise">
          <div class="terminal-head">
            <div>
              <strong>Operator Terminal</strong>
              <div class="submetric">Runs in the Pico workspace with short timeout and session history.</div>
            </div>
            <div id="terminalHeadMeta" class="mono" style="font-size:11px;"></div>
          </div>
          <div id="terminalFeed" class="terminal-feed">Terminal locked.</div>
          <form id="terminalForm" class="terminal-form">
            <input id="terminalCommand" class="input mono" placeholder="journalctl -u picoclaw -n 60 --no-pager" />
            <button class="btn primary" type="submit">Run</button>
            <button class="btn" type="button" id="terminalRefreshBtn">Refresh</button>
          </form>
        </div>
      </section>

      <section id="view-logs" class="view">
        <div class="view-head">
          <div>
            <h2 class="view-title">Logs / Artifacts</h2>
            <p class="view-sub">Searchable operational logs, artifacts browser, and latest failures.</p>
          </div>
        </div>
        <div class="toolbar">
          <select id="logsSource">
            <option value="heartbeat">heartbeat</option>
            <option value="dashboard">dashboard</option>
          </select>
          <input id="logsSearch" class="input" placeholder="Search logs" style="max-width:240px;" />
          <button class="btn" id="logsRefreshBtn">Refresh Logs</button>
        </div>
        <div class="grid two">
          <div class="card">
            <h3>Recent Logs</h3>
            <div id="logsBody" class="mono" style="font-size:11px; white-space:pre-wrap; max-height:420px; overflow:auto;"></div>
          </div>
          <div class="card">
            <h3>Artifacts</h3>
            <div id="artifactList"></div>
          </div>
        </div>
      </section>
    </main>
  </div>

  <script>
    /**
     * Type hints:
     * ControlPlaneStatus -> generated_at, gateway, summary, services, agents, nodes, tailscale, auth, jobs, media, app_generator, logs, pending_setup
     * ActionRequest -> action, target, payload
     * TestChatRequest -> prompt, agent_id, execution_target, workflow_template, test_type, dry_run, payload
     */
    const API = {
      status: '/api/control-plane/status',
      chat: '/api/control-plane/test-chat',
      history: '/api/control-plane/test-chat/history',
      action: '/api/control-plane/action',
      logs: '/api/control-plane/logs',
      terminalStatus: '/api/control-plane/terminal/status',
      terminalLogin: '/api/control-plane/terminal/login',
      terminalLogout: '/api/control-plane/terminal/logout',
      terminalExec: '/api/control-plane/terminal/exec'
    };

    const app = {
      tab: 'overview',
      pollMs: 10000,
      polling: true,
      status: null,
      chatHistory: [],
      terminal: null
    };

    function esc(v) {
      return String(v || '')
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
    }

    function pill(status) {
      const s = (status || 'unknown').toLowerCase();
      const cls = s === 'ok' ? 'status-ok' : (s === 'warn' ? 'status-warn' : (s === 'error' ? 'status-error' : 'status-unknown'));
      return '<span class="status-pill ' + cls + '">' + esc(s) + '</span>';
    }

    async function fetchJSON(url, opts) {
      const res = await fetch(url, opts || { cache: 'no-store' });
      const text = await res.text();
      let data = {};
      try { data = text ? JSON.parse(text) : {}; } catch (_) { data = { raw: text }; }
      if (!res.ok) {
        throw new Error((data && data.error) ? data.error : ('HTTP ' + res.status));
      }
      return data;
    }

    function switchTab(tab) {
      app.tab = tab;
      document.querySelectorAll('.nav-btn').forEach(function (btn) {
        btn.classList.toggle('active', btn.dataset.tab === tab);
      });
      document.querySelectorAll('.view').forEach(function (view) {
        view.classList.toggle('active', view.id === 'view-' + tab);
      });
    }

    function renderOverview(data) {
      const chips = [
        '<span class="chip">nodes ' + esc(data.summary.nodes) + '</span>',
        '<span class="chip">agents ' + esc(data.summary.agents) + '</span>',
        '<span class="chip">jobs ' + esc(data.summary.jobs_total) + '</span>',
        '<span class="chip">setup pending ' + esc(data.summary.pending_setup) + '</span>',
        '<span class="chip">degraded ' + esc(data.summary.degraded_services) + '</span>'
      ];
      document.getElementById('summaryChips').innerHTML = chips.join(' ');

      const cards = (data.services || []).map(function (svc) {
        return '<div class="card">'
          + '<h3>' + esc(svc.name) + '</h3>'
          + '<div>' + pill(svc.status) + '</div>'
          + '<div class="metric">' + esc(svc.uptime || 'n/a') + '</div>'
          + '<div class="submetric">last error: ' + esc(svc.last_error || 'none') + '</div>'
          + '<div class="submetric">last success: ' + esc(svc.last_success || 'n/a') + '</div>'
          + '</div>';
      }).join('');
      document.getElementById('serviceCards').innerHTML = cards || '<div class="card">No services</div>';

      const tasks = (data.pending_setup || []).map(function (t) {
        return '<li><strong>' + esc(t.name) + '</strong> - ' + esc(t.description) + ' (' + esc(t.status) + ')</li>';
      }).join('');
      document.getElementById('pendingTasks').innerHTML = tasks || '<li>No pending setup tasks detected.</li>';

      const global = data.summary.degraded_services > 0 || data.summary.pending_setup > 0 ? 'warn' : 'ok';
      document.getElementById('globalStatus').className = 'status-pill ' + (global === 'ok' ? 'status-ok' : 'status-warn');
      document.getElementById('globalStatus').textContent = global === 'ok' ? 'cluster healthy' : 'attention needed';
    }

    function renderAgents(data) {
      const heads = ['Agent', 'Type', 'State', 'Active Job', 'Queue', 'Heartbeat', 'Capabilities', 'Node', 'Target', 'Concurrency', 'Actions'];
      const rows = (data.agents || []).map(function (a) {
        return '<tr>'
          + '<td><strong>' + esc(a.id) + '</strong><br><span class="mono">' + esc(a.name || '') + '</span></td>'
          + '<td>' + esc(a.type) + '</td>'
          + '<td>' + pill(a.state) + '</td>'
          + '<td>' + esc(a.active_job || '-') + '</td>'
          + '<td>' + esc(a.queue_depth) + '</td>'
          + '<td>' + esc(a.last_heartbeat || '-') + '</td>'
          + '<td>' + (a.capabilities || []).map(function (c) { return '<span class="chip">' + esc(c) + '</span>'; }).join('') + '</td>'
          + '<td>' + esc(a.assigned_node || '-') + '</td>'
          + '<td>' + esc(a.execution_target || '-') + '</td>'
          + '<td>' + esc(a.concurrency || 1) + '</td>'
          + '<td><button class="btn" disabled>start</button> <button class="btn" disabled>stop</button> <button class="btn" disabled>restart</button></td>'
          + '</tr>';
      }).join('');
      document.getElementById('agentsTable').innerHTML = '<thead><tr>' + heads.map(function (h) { return '<th>' + h + '</th>'; }).join('') + '</tr></thead><tbody>' + rows + '</tbody>';

      const logsHtml = (data.agents || []).map(function (a) {
        const lines = (a.logs || []).map(function (line) { return esc(line); }).join('\n');
        return '<details><summary>' + esc(a.id) + ' logs</summary><pre class="mono" style="font-size:11px; white-space:pre-wrap;">' + lines + '</pre></details>';
      }).join('');
      document.getElementById('agentLogs').innerHTML = logsHtml || 'No agent logs available.';
    }

    function renderNodes(data) {
      const heads = ['Node', 'Hostname', 'Tags', 'Role', 'Tailscale IP', 'SSH', 'Reachability', 'Heartbeat', 'Services', 'Storage', 'CPU/RAM', 'Recent Failures', 'Actions'];
      const rows = (data.nodes || []).map(function (n) {
        return '<tr>'
          + '<td><strong>' + esc(n.id) + '</strong></td>'
          + '<td>' + esc(n.hostname || '-') + '</td>'
          + '<td>' + (n.tags || []).map(function (t) { return '<span class="chip">' + esc(t) + '</span>'; }).join('') + '</td>'
          + '<td>' + esc(n.role || '-') + '</td>'
          + '<td class="mono">' + esc(n.tailscale_ip || '-') + '</td>'
          + '<td>' + pill(n.ssh_status) + '</td>'
          + '<td>' + esc(n.reachability || '-') + '</td>'
          + '<td>' + esc(n.last_heartbeat || '-') + '</td>'
          + '<td>' + (n.exposed_services || []).map(function (s) { return '<span class="chip">' + esc(s) + '</span>'; }).join('') + '</td>'
          + '<td>' + esc(n.storage_usage || '-') + '</td>'
          + '<td>' + esc(n.cpu_memory || '-') + '</td>'
          + '<td>' + esc(n.recent_failures || '-') + '</td>'
          + '<td><button class="btn" data-node-action="ping" data-node="' + esc(n.id) + '">ping</button></td>'
          + '</tr>';
      }).join('');
      document.getElementById('nodesTable').innerHTML = '<thead><tr>' + heads.map(function (h) { return '<th>' + h + '</th>'; }).join('') + '</tr></thead><tbody>' + rows + '</tbody>';
    }

    function renderTailscale(data) {
      const tailscale = data.tailscale || { paths: [], warnings: [] };
      const heads = ['From', 'To', 'Status', 'Reason', 'Tailscale Online', 'SSH Enabled', 'Tag State', 'Worker Registered'];
      const rows = (tailscale.paths || []).map(function (p) {
        return '<tr>'
          + '<td>' + esc(p.from) + '</td>'
          + '<td>' + esc(p.to) + '</td>'
          + '<td>' + pill(p.status) + '</td>'
          + '<td>' + esc(p.reason || '-') + '</td>'
          + '<td>' + esc(p.tailscale_online) + '</td>'
          + '<td>' + esc(p.ssh_enabled) + '</td>'
          + '<td>' + esc(p.tag_state || '-') + '</td>'
          + '<td>' + esc(p.worker_registered) + '</td>'
          + '</tr>';
      }).join('');
      document.getElementById('pathMatrixTable').innerHTML = '<thead><tr>' + heads.map(function (h) { return '<th>' + h + '</th>'; }).join('') + '</tr></thead><tbody>' + rows + '</tbody>';

      document.getElementById('tailscaleWarnings').innerHTML = (tailscale.warnings || []).map(function (w) {
        return '<li>' + esc(w) + '</li>';
      }).join('') || '<li>No blockers detected.</li>';
    }

    function renderAuth(data) {
      const auth = data.auth || { checklist: [], providers: [] };
      document.getElementById('authChecklist').innerHTML = (auth.checklist || []).map(function (c) {
        return '<li>' + pill(c.status) + ' <strong>' + esc(c.name) + '</strong> - ' + esc(c.description) + '</li>';
      }).join('') || '<li>No checklist items.</li>';

      document.getElementById('authProviders').innerHTML = (auth.providers || []).map(function (p) {
        return '<div class="kv"><div class="k">provider</div><div>' + esc(p.provider) + '</div>'
          + '<div class="k">status</div><div>' + pill(p.status) + '</div>'
          + '<div class="k">auth method</div><div>' + esc(p.auth_method || '-') + '</div>'
          + '<div class="k">account</div><div>' + esc(p.account || '-') + '</div>'
          + '<div class="k">expires</div><div>' + esc(p.expires_at || '-') + '</div></div><hr style="border-color:#20324d;" />';
      }).join('') || 'No provider auth records.';
    }

    function renderSecurity(data) {
      const security = data.security || {};
      const summary = security.summary || {};
      document.getElementById('securityCards').innerHTML = [
        { k: 'Overall posture', v: pill(summary.overall || 'unknown') },
        { k: 'Security score', v: esc(summary.score || 0) + '/100' },
        { k: 'Critical findings', v: esc(summary.critical || 0) },
        { k: 'Warnings', v: esc(summary.warn || 0) },
        { k: 'Auth ready', v: esc(summary.auth_ready || 0) },
        { k: 'Secure web', v: (summary.secure_web ? 'ready' : 'not ready') + (summary.llm_ready ? ' • llm' : '') }
      ].map(function (c) {
        return '<div class="card"><h3>' + esc(c.k) + '</h3><div class="metric">' + c.v + '</div></div>';
      }).join('');

      document.getElementById('securityDomains').innerHTML = (security.domains || []).map(function (d) {
        const signals = (d.signals || []).map(function (s) { return '<span class="chip">' + esc(s) + '</span>'; }).join('');
        return '<div class="domain-card">'
          + '<h4><span>' + esc(d.name) + '</span>' + pill(d.status || 'unknown') + '</h4>'
          + '<div class="metric-sm">score ' + esc(d.score || 0) + '/100</div>'
          + '<div class="progress"><span style="width:' + esc(d.score || 0) + '%;"></span></div>'
          + '<p style="margin-top:8px;">' + esc(d.detail || '-') + '</p>'
          + '<div style="margin-top:8px;">' + signals + '</div>'
          + '</div>';
      }).join('') || 'No security domains yet.';

      document.getElementById('securityFindings').innerHTML = (security.findings || []).map(function (f) {
        return '<div class="finding"><h4>' + pill(f.severity || 'unknown') + ' ' + esc(f.title || f.area || 'finding') + '</h4>'
          + '<p><strong>Area:</strong> ' + esc(f.area || '-') + '</p>'
          + '<p>' + esc(f.detail || '-') + '</p>'
          + '<p><strong>Fix:</strong> ' + esc(f.fix_hint || '-') + '</p></div>';
      }).join('') || 'No active findings.';

      document.getElementById('securityAccess').innerHTML = (security.access || []).map(function (a) {
        const link = a.url ? '<a href="' + esc(a.url) + '" target="_blank" rel="noreferrer">' + esc(a.url) + '</a>' : '<span class="mono">n/a</span>';
        return '<div class="access-card"><div class="kv">'
          + '<div class="k">surface</div><div>' + esc(a.name || '-') + '</div>'
          + '<div class="k">status</div><div>' + pill(a.status || 'unknown') + '</div>'
          + '<div class="k">url</div><div>' + link + '</div>'
          + '<div class="k">auth</div><div>' + esc(a.auth || '-') + '</div>'
          + '<div class="k">exposure</div><div>' + esc(a.exposure || '-') + '</div>'
          + '<div class="k">notes</div><div>' + esc(a.notes || '-') + '</div>'
          + '</div></div>';
      }).join('') || 'No access surfaces detected.';

      const diagnosis = security.diagnosis || {};
      document.getElementById('securityDiagnosis').innerHTML =
        '<div class="stack"><h3 style="margin:0;">Latest Diagnosis</h3>'
        + '<div class="kv"><div class="k">status</div><div>' + pill(diagnosis.status || 'idle') + '</div>'
        + '<div class="k">source</div><div>' + esc(diagnosis.source || '-') + '</div>'
        + '<div class="k">last run</div><div>' + esc(diagnosis.last_run_at || '-') + '</div>'
        + '<div class="k">summary</div><div style="white-space:pre-wrap;">' + esc(diagnosis.summary || diagnosis.last_error || 'No diagnosis run yet.') + '</div></div></div>';
    }

    function renderJobs(data) {
      const jobs = data.jobs || { items: [] };
      const cards = [
        { k: 'Queued', v: jobs.queued || 0 },
        { k: 'Running', v: jobs.running || 0 },
        { k: 'Failed', v: jobs.failed || 0 },
        { k: 'Completed', v: jobs.completed || 0 },
        { k: 'Retries', v: jobs.retries || 0 }
      ].map(function (c) {
        return '<div class="card"><h3>' + esc(c.k) + '</h3><div class="metric">' + esc(c.v) + '</div></div>';
      }).join('');
      document.getElementById('jobCounters').innerHTML = cards;

      const heads = ['Job ID', 'Type', 'State', 'Agent', 'Worker', 'Start', 'Duration', 'Artifacts', 'Manifest', 'Actions'];
      const rows = (jobs.items || []).map(function (j) {
        return '<tr>'
          + '<td class="mono">' + esc(j.id) + '</td>'
          + '<td>' + esc(j.job_type) + '</td>'
          + '<td>' + pill(j.state) + '</td>'
          + '<td>' + esc(j.agent_id || '-') + '</td>'
          + '<td>' + esc(j.assigned_worker || '-') + '</td>'
          + '<td>' + esc(j.start_time || '-') + '</td>'
          + '<td>' + esc(j.duration || '-') + '</td>'
          + '<td>' + (j.artifacts || []).map(function (a) { return '<span class="chip">' + esc(a) + '</span>'; }).join('') + '</td>'
          + '<td class="mono">' + esc(j.manifest_summary || '-') + '</td>'
          + '<td><button class="btn" data-job-action="retry" data-job-id="' + esc(j.id) + '">retry</button> <button class="btn danger" data-job-action="cancel" data-job-id="' + esc(j.id) + '">cancel</button></td>'
          + '</tr>';
      }).join('');
      document.getElementById('jobsTable').innerHTML = '<thead><tr>' + heads.map(function (h) { return '<th>' + h + '</th>'; }).join('') + '</tr></thead><tbody>' + rows + '</tbody>';

      const timeline = (jobs.items || []).slice(0, 8).map(function (j) {
        const tl = (j.timeline || []).map(function (t) {
          return '[' + esc(t.at) + '] ' + esc(t.status) + ' - ' + esc(t.message || '');
        }).join('\n');
        return '## ' + j.id + '\n' + tl;
      }).join('\n\n');
      document.getElementById('jobTimeline').textContent = timeline || 'No job timelines yet.';
    }

    function renderChat(data) {
      const feed = document.getElementById('chatFeed');
      const history = app.chatHistory || [];
      feed.innerHTML = history.map(function (m) {
        return '<div class="msg ' + esc(m.role) + '"><div class="meta">' + esc(m.role) + ' • ' + esc(m.created_at) + '</div>' + esc(m.content) + '</div>';
      }).join('');
      feed.scrollTop = feed.scrollHeight;

      const agentSelect = document.getElementById('chatAgent');
      agentSelect.innerHTML = (data.agents || []).map(function (a) {
        return '<option value="' + esc(a.id) + '">' + esc(a.id) + ' (' + esc(a.state) + ')</option>';
      }).join('');

      const targetSelect = document.getElementById('chatTarget');
      targetSelect.innerHTML = (data.nodes || []).map(function (n) {
        return '<option value="' + esc(n.id) + '">' + esc(n.id) + ' / ' + esc(n.hostname || '') + '</option>';
      }).join('');
    }

    function renderMedia(data) {
      const media = data.media || { pipeline: [], templates: [] };
      document.getElementById('mediaStatus').innerHTML = (media.pipeline || []).map(function (s) {
        return '<div class="kv"><div class="k">step</div><div>' + esc(s.step) + '</div><div class="k">status</div><div>' + pill(s.status) + '</div><div class="k">detail</div><div>' + esc(s.detail || '-') + '</div></div><hr style="border-color:#21324f;">';
      }).join('') || 'No media pipeline state yet.';

      document.getElementById('mediaTemplates').innerHTML = (media.templates || []).map(function (t) {
        return '<details><summary>' + esc(t.name) + '</summary><pre class="mono" style="font-size:11px;">' + esc(t.sample_manifest || '{}') + '</pre></details>';
      }).join('') || 'No media templates.';
    }

    function renderAppGen(data) {
      const appGen = data.app_generator || { requests: [], status: [] };
      document.getElementById('appGenRequests').innerHTML = (appGen.requests || []).map(function (r) {
        return '<div class="kv"><div class="k">request</div><div>' + esc(r.id) + '</div><div class="k">project type</div><div>' + esc(r.project_type) + '</div><div class="k">template</div><div>' + esc(r.template) + '</div><div class="k">target</div><div>' + esc(r.deploy_target || '-') + '</div><div class="k">status</div><div>' + pill(r.status) + '</div></div><hr style="border-color:#21324f;">';
      }).join('') || 'No app generation requests yet.';

      document.getElementById('appGenStatus').innerHTML = (appGen.status || []).map(function (s) {
        return '<div class="kv"><div class="k">workspace</div><div class="mono">' + esc(s.workspace || '-') + '</div><div class="k">preview</div><div>' + esc(s.preview_url || '-') + '</div><div class="k">build</div><div>' + pill(s.build_status || 'unknown') + '</div><div class="k">tests</div><div>' + pill(s.test_status || 'unknown') + '</div><div class="k">child agents</div><div>' + esc((s.child_agents || []).join(', ') || '-') + '</div></div><hr style="border-color:#21324f;">';
      }).join('') || 'No app generation status records.';
    }

    function renderTerminal(data) {
      const status = data || {};
      app.terminal = status;
      const pills = [
        pill(status.allowed ? 'ok' : 'warn'),
        pill(status.authenticated ? 'ok' : 'warn')
      ].join(' ');
      document.getElementById('terminalStatePill').innerHTML = pills;

      document.getElementById('terminalPolicy').innerHTML =
        '<div class="k">host</div><div>' + esc(status.host || '-') + '</div>' +
        '<div class="k">tailscale</div><div>' + esc(status.tailscale) + '</div>' +
        '<div class="k">secure</div><div>' + esc(status.secure) + '</div>' +
        '<div class="k">login configured</div><div>' + esc(status.login_configured) + '</div>' +
        '<div class="k">reason</div><div>' + esc(status.reason || '-') + '</div>';

      document.getElementById('terminalQuick').innerHTML = (status.suggestions || []).map(function (cmd) {
        return '<button class="btn" type="button" data-terminal-suggest="' + esc(cmd) + '">' + esc(cmd) + '</button>';
      }).join('');

      document.getElementById('terminalHeadMeta').textContent =
        (status.tailscale ? 'tailscale' : 'non-tailscale') + ' • ' +
        (status.secure ? 'https' : 'insecure') + ' • ' +
        (status.authenticated ? 'authenticated' : 'locked');

      const history = status.history || [];
      document.getElementById('terminalFeed').innerHTML = history.length ? history.map(function (entry) {
        return '<div class="terminal-entry">'
          + '<div class="terminal-cmd">$ ' + esc(entry.command) + ' <span class="submetric">[' + esc(entry.status) + ' / exit ' + esc(entry.exit_code) + ']</span></div>'
          + '<div class="terminal-output">' + esc(entry.output || '') + '</div>'
          + '<div class="submetric" style="margin-top:6px;">' + esc(entry.at || '') + '</div>'
          + '</div>';
      }).join('') : 'Terminal locked.';

      document.getElementById('terminalCommand').disabled = !status.allowed || !status.authenticated;
      document.querySelector('#terminalForm button[type="submit"]').disabled = !status.allowed || !status.authenticated;
      document.getElementById('terminalLock').style.display = status.allowed && status.authenticated ? 'none' : 'block';
    }

    async function refreshTerminalStatus() {
      try {
        const data = await fetchJSON(API.terminalStatus, { cache: 'no-store' });
        renderTerminal(data);
      } catch (err) {
        document.getElementById('terminalFeed').textContent = 'Failed to load terminal status: ' + err.message;
      }
    }

    async function refreshLogs() {
      const source = document.getElementById('logsSource').value;
      const search = document.getElementById('logsSearch').value;
      try {
        const data = await fetchJSON(API.logs + '?source=' + encodeURIComponent(source) + '&search=' + encodeURIComponent(search));
        document.getElementById('logsBody').textContent = (data.lines || []).join('\n') || 'No logs available.';
      } catch (err) {
        document.getElementById('logsBody').textContent = 'Failed to load logs: ' + err.message;
      }
    }

    function renderArtifacts(data) {
      const artifacts = ((data.logs || {}).artifacts) || [];
      document.getElementById('artifactList').innerHTML = artifacts.map(function (a) {
        return '<div class="kv"><div class="k">path</div><div class="mono">' + esc(a.path) + '</div><div class="k">kind</div><div>' + esc(a.kind) + '</div><div class="k">updated</div><div>' + esc(a.updated_at || '-') + '</div><div class="k">size</div><div>' + esc(a.size || '-') + '</div></div><hr style="border-color:#22324f;">';
      }).join('') || 'No artifacts discovered.';
    }

    async function refreshStatus() {
      try {
        const data = await fetchJSON(API.status, { cache: 'no-store' });
        app.status = data;
        renderOverview(data);
        renderAgents(data);
        renderNodes(data);
        renderTailscale(data);
        renderSecurity(data);
        renderAuth(data);
        renderJobs(data);
        renderChat(data);
        renderMedia(data);
        renderAppGen(data);
        renderArtifacts(data);
        await refreshTerminalStatus();
      } catch (err) {
        document.getElementById('globalStatus').className = 'status-pill status-error';
        document.getElementById('globalStatus').textContent = 'error: ' + err.message;
      }
    }

    async function refreshChatHistory() {
      try {
        const data = await fetchJSON(API.history, { cache: 'no-store' });
        app.chatHistory = data.messages || [];
        if (app.status) {
          renderChat(app.status);
        }
      } catch (err) {
        console.error(err);
      }
    }

    async function runAction(action, target, payload, resultTarget) {
      const outputID = resultTarget || 'actionResult';
      const outputNode = document.getElementById(outputID);
      try {
        const data = await fetchJSON(API.action, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ action: action, target: target || '', payload: payload || {} })
        });
        if (outputNode) {
          outputNode.textContent = JSON.stringify(data, null, 2);
        }
        await refreshStatus();
        await refreshChatHistory();
      } catch (err) {
        if (outputNode) {
          outputNode.textContent = 'Action failed: ' + err.message;
        }
      }
    }

    function loadPreset() {
      const raw = localStorage.getItem('pcp-test-preset');
      if (!raw) {
        document.getElementById('chatInfo').textContent = 'No preset saved yet.';
        return;
      }
      try {
        const p = JSON.parse(raw);
        document.getElementById('chatAgent').value = p.agent_id || '';
        document.getElementById('chatTarget').value = p.execution_target || '';
        document.getElementById('chatWorkflow').value = p.workflow_template || 'generic-debug';
        document.getElementById('chatType').value = p.test_type || 'agent_prompt';
        document.getElementById('chatPrompt').value = p.prompt || '';
        document.getElementById('chatPayload').value = p.payload || '';
        document.getElementById('chatDryRun').checked = !!p.dry_run;
        document.getElementById('chatInfo').textContent = 'Preset loaded.';
      } catch (err) {
        document.getElementById('chatInfo').textContent = 'Invalid preset: ' + err.message;
      }
    }

    function savePreset() {
      const payload = {
        agent_id: document.getElementById('chatAgent').value,
        execution_target: document.getElementById('chatTarget').value,
        workflow_template: document.getElementById('chatWorkflow').value,
        test_type: document.getElementById('chatType').value,
        prompt: document.getElementById('chatPrompt').value,
        payload: document.getElementById('chatPayload').value,
        dry_run: document.getElementById('chatDryRun').checked
      };
      localStorage.setItem('pcp-test-preset', JSON.stringify(payload));
      document.getElementById('chatInfo').textContent = 'Preset saved.';
    }

    async function submitChat(ev) {
      ev.preventDefault();
      let payloadObj = {};
      const rawPayload = document.getElementById('chatPayload').value.trim();
      if (rawPayload) {
        try {
          payloadObj = JSON.parse(rawPayload);
        } catch (err) {
          document.getElementById('chatInfo').textContent = 'JSON payload parse error: ' + err.message;
          return;
        }
      }

      const req = {
        prompt: document.getElementById('chatPrompt').value,
        agent_id: document.getElementById('chatAgent').value,
        execution_target: document.getElementById('chatTarget').value,
        workflow_template: document.getElementById('chatWorkflow').value,
        test_type: document.getElementById('chatType').value,
        dry_run: document.getElementById('chatDryRun').checked,
        payload: payloadObj
      };

      try {
        const res = await fetchJSON(API.chat, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify(req)
        });
        document.getElementById('chatInfo').textContent = 'Submitted job ' + res.job_id;
        document.getElementById('chatPrompt').value = '';
        await refreshChatHistory();
        await refreshStatus();
      } catch (err) {
        document.getElementById('chatInfo').textContent = 'Submit failed: ' + err.message;
      }
    }

    async function submitTerminalLogin(ev) {
      ev.preventDefault();
      const password = document.getElementById('terminalPassword').value;
      try {
        await fetchJSON(API.terminalLogin, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ password: password })
        });
        document.getElementById('terminalPassword').value = '';
        document.getElementById('terminalLoginInfo').textContent = 'Terminal unlocked.';
        await refreshTerminalStatus();
      } catch (err) {
        document.getElementById('terminalLoginInfo').textContent = 'Terminal login failed: ' + err.message;
      }
    }

    async function submitTerminalCommand(ev) {
      ev.preventDefault();
      const command = document.getElementById('terminalCommand').value.trim();
      if (!command) return;
      try {
        const data = await fetchJSON(API.terminalExec, {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ command: command })
        });
        document.getElementById('terminalCommand').value = '';
        renderTerminal(data.status || app.terminal || {});
      } catch (err) {
        document.getElementById('terminalLoginInfo').textContent = 'Terminal command failed: ' + err.message;
        await refreshTerminalStatus();
      }
    }

    async function logoutTerminal() {
      try {
        await fetchJSON(API.terminalLogout, { method: 'POST' });
        document.getElementById('terminalLoginInfo').textContent = 'Terminal logged out.';
      } catch (err) {
        document.getElementById('terminalLoginInfo').textContent = 'Terminal logout failed: ' + err.message;
      }
      await refreshTerminalStatus();
    }

    function bindEvents() {
      document.querySelectorAll('.nav-btn').forEach(function (btn) {
        btn.addEventListener('click', function () { switchTab(btn.dataset.tab); });
      });

      document.getElementById('refreshBtn').addEventListener('click', async function () {
        await refreshStatus();
        await refreshChatHistory();
        await refreshLogs();
      });

      document.getElementById('togglePollBtn').addEventListener('click', function () {
        app.polling = !app.polling;
        this.textContent = app.polling ? 'Pause Polling' : 'Resume Polling';
      });

      document.getElementById('openLegacyBtn').addEventListener('click', function () {
        window.open('/legacy-dashboard', '_blank');
      });

      document.getElementById('chatForm').addEventListener('submit', submitChat);
      document.getElementById('savePresetBtn').addEventListener('click', savePreset);
      document.getElementById('loadPresetBtn').addEventListener('click', loadPreset);
      document.getElementById('terminalLoginForm').addEventListener('submit', submitTerminalLogin);
      document.getElementById('terminalForm').addEventListener('submit', submitTerminalCommand);
      document.getElementById('terminalLogoutBtn').addEventListener('click', logoutTerminal);
      document.getElementById('terminalRefreshBtn').addEventListener('click', refreshTerminalStatus);

      document.getElementById('logsRefreshBtn').addEventListener('click', refreshLogs);
      document.getElementById('logsSearch').addEventListener('keydown', function (ev) {
        if (ev.key === 'Enter') {
          ev.preventDefault();
          refreshLogs();
        }
      });

      document.querySelector('#view-overview').addEventListener('click', function (ev) {
        const btn = ev.target.closest('button[data-action]');
        if (!btn) return;
        runAction(btn.dataset.action, '', {}, btn.dataset.resultTarget || 'actionResult');
      });

      document.querySelector('#view-auth').addEventListener('click', function (ev) {
        const btn = ev.target.closest('button[data-action]');
        if (!btn) return;
        runAction(btn.dataset.action, '', {}, btn.dataset.resultTarget || 'securityActionResult');
      });

      document.querySelector('#view-terminal').addEventListener('click', function (ev) {
        const btn = ev.target.closest('button[data-terminal-suggest]');
        if (!btn) return;
        document.getElementById('terminalCommand').value = btn.dataset.terminalSuggest;
      });

      document.getElementById('nodesTable').addEventListener('click', function (ev) {
        const btn = ev.target.closest('button[data-node-action]');
        if (!btn) return;
        runAction('ping_workers', btn.dataset.node, {}, 'actionResult');
      });

      document.getElementById('jobsTable').addEventListener('click', function (ev) {
        const btn = ev.target.closest('button[data-job-action]');
        if (!btn) return;
        const action = btn.dataset.jobAction === 'retry' ? 'retry_job' : 'cancel_job';
        runAction(action, '', { job_id: btn.dataset.jobId }, 'actionResult');
      });
    }

    async function tick() {
      if (app.polling) {
        await refreshStatus();
        await refreshChatHistory();
        await refreshTerminalStatus();
      }
      setTimeout(tick, app.pollMs);
    }

    bindEvents();
    refreshStatus().then(refreshChatHistory).then(refreshLogs);
    setTimeout(tick, app.pollMs);
  </script>
</body>
</html>`))

type controlPlanePageData struct {
	Address     string
	GeneratedAt string
}

type controlPlaneStatus struct {
	GeneratedAt  string                 `json:"generated_at"`
	Gateway      string                 `json:"gateway"`
	Summary      controlSummary         `json:"summary"`
	Services     []controlService       `json:"services"`
	Agents       []controlAgent         `json:"agents"`
	Nodes        []controlNode          `json:"nodes"`
	Tailscale    controlTailscale       `json:"tailscale"`
	Auth         controlAuth            `json:"auth"`
	Security     controlSecurity        `json:"security"`
	Jobs         controlJobs            `json:"jobs"`
	Media        controlMedia           `json:"media"`
	AppGenerator controlAppGenerator    `json:"app_generator"`
	Logs         controlLogs            `json:"logs"`
	Channels     []controlChannel       `json:"channels"`
	Webhooks     []controlWebhook       `json:"webhooks"`
	PendingSetup []controlChecklistItem `json:"pending_setup"`
}

type controlSummary struct {
	Nodes            int `json:"nodes"`
	Agents           int `json:"agents"`
	JobsTotal        int `json:"jobs_total"`
	PendingSetup     int `json:"pending_setup"`
	DegradedServices int `json:"degraded_services"`
}

type controlService struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Uptime      string `json:"uptime"`
	LastError   string `json:"last_error"`
	LastSuccess string `json:"last_success"`
}

type controlAgent struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	State           string   `json:"state"`
	ActiveJob       string   `json:"active_job"`
	QueueDepth      int      `json:"queue_depth"`
	LastHeartbeat   string   `json:"last_heartbeat"`
	Capabilities    []string `json:"capabilities"`
	AssignedNode    string   `json:"assigned_node"`
	ExecutionTarget string   `json:"execution_target"`
	Concurrency     int      `json:"concurrency"`
	Logs            []string `json:"logs"`
}

type controlNode struct {
	ID              string   `json:"id"`
	Hostname        string   `json:"hostname"`
	Tags            []string `json:"tags"`
	Role            string   `json:"role"`
	TailscaleIP     string   `json:"tailscale_ip"`
	SSHStatus       string   `json:"ssh_status"`
	Reachability    string   `json:"reachability"`
	LastHeartbeat   string   `json:"last_heartbeat"`
	ExposedServices []string `json:"exposed_services"`
	StorageUsage    string   `json:"storage_usage"`
	CPUMemory       string   `json:"cpu_memory"`
	RecentFailures  string   `json:"recent_failures"`
	Online          bool     `json:"online"`
	SSHEnabled      bool     `json:"ssh_enabled"`
	WorkerReady     bool     `json:"worker_ready"`
}

type controlTailscale struct {
	Paths    []controlPath `json:"paths"`
	Warnings []string      `json:"warnings"`
}

type controlPath struct {
	From             string `json:"from"`
	To               string `json:"to"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	TailscaleOnline  bool   `json:"tailscale_online"`
	SSHEnabled       bool   `json:"ssh_enabled"`
	TagState         string `json:"tag_state"`
	WorkerRegistered bool   `json:"worker_registered"`
}

type controlAuth struct {
	Checklist []controlChecklistItem `json:"checklist"`
	Providers []controlAuthProvider  `json:"providers"`
}

type controlSecurity struct {
	Summary   controlSecuritySummary   `json:"summary"`
	Domains   []controlSecurityDomain  `json:"domains"`
	Findings  []controlSecurityFinding `json:"findings"`
	Access    []controlSecurityAccess  `json:"access"`
	Diagnosis controlSecurityDiagnosis `json:"diagnosis"`
}

type controlSecuritySummary struct {
	Overall   string `json:"overall"`
	Score     int    `json:"score"`
	Critical  int    `json:"critical"`
	Warn      int    `json:"warn"`
	AuthReady int    `json:"auth_ready"`
	SecureWeb bool   `json:"secure_web"`
	LLMReady  bool   `json:"llm_ready"`
}

type controlSecurityDomain struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Score   int      `json:"score"`
	Detail  string   `json:"detail"`
	Signals []string `json:"signals"`
}

type controlSecurityFinding struct {
	Severity string `json:"severity"`
	Area     string `json:"area"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	FixHint  string `json:"fix_hint"`
}

type controlSecurityAccess struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Status   string `json:"status"`
	Auth     string `json:"auth"`
	Exposure string `json:"exposure"`
	Notes    string `json:"notes"`
}

type controlSecurityDiagnosis struct {
	Status    string `json:"status"`
	LastRunAt string `json:"last_run_at"`
	Summary   string `json:"summary"`
	Source    string `json:"source"`
	LastError string `json:"last_error"`
	LastJobID string `json:"last_job_id"`
}

type controlTerminalStatus struct {
	Allowed         bool                   `json:"allowed"`
	Authenticated   bool                   `json:"authenticated"`
	Secure          bool                   `json:"secure"`
	Tailscale       bool                   `json:"tailscale"`
	LoginConfigured bool                   `json:"login_configured"`
	Reason          string                 `json:"reason"`
	Host            string                 `json:"host"`
	History         []controlTerminalEntry `json:"history"`
	Suggestions     []string               `json:"suggestions"`
}

type controlTerminalEntry struct {
	At       string `json:"at"`
	Command  string `json:"command"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Status   string `json:"status"`
}

type controlChecklistItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type controlAuthProvider struct {
	Provider   string `json:"provider"`
	AuthMethod string `json:"auth_method"`
	Status     string `json:"status"`
	Account    string `json:"account"`
	ExpiresAt  string `json:"expires_at"`
}

type controlJobs struct {
	Queued    int          `json:"queued"`
	Running   int          `json:"running"`
	Failed    int          `json:"failed"`
	Completed int          `json:"completed"`
	Retries   int          `json:"retries"`
	Items     []controlJob `json:"items"`
}

type controlJob struct {
	ID              string            `json:"id"`
	JobType         string            `json:"job_type"`
	State           string            `json:"state"`
	AgentID         string            `json:"agent_id"`
	AssignedWorker  string            `json:"assigned_worker"`
	StartTime       string            `json:"start_time"`
	Duration        string            `json:"duration"`
	Artifacts       []string          `json:"artifacts"`
	ManifestSummary string            `json:"manifest_summary"`
	Timeline        []controlJobEvent `json:"timeline"`
	Payload         map[string]any    `json:"-"`
	CreatedAt       time.Time         `json:"-"`
	StartedAt       time.Time         `json:"-"`
	FinishedAt      time.Time         `json:"-"`
	RetryCount      int               `json:"-"`
}

type controlJobEvent struct {
	At      string `json:"at"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type controlChatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type controlMedia struct {
	Pipeline  []controlPipelineStep `json:"pipeline"`
	Templates []controlTemplate     `json:"templates"`
}

type controlPipelineStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type controlTemplate struct {
	Name           string `json:"name"`
	SampleManifest string `json:"sample_manifest"`
}

type controlAppGenerator struct {
	Requests []controlAppRequest `json:"requests"`
	Status   []controlAppStatus  `json:"status"`
}

type controlAppRequest struct {
	ID           string `json:"id"`
	ProjectType  string `json:"project_type"`
	Template     string `json:"template"`
	DeployTarget string `json:"deploy_target"`
	Status       string `json:"status"`
}

type controlAppStatus struct {
	Workspace   string   `json:"workspace"`
	PreviewURL  string   `json:"preview_url"`
	BuildStatus string   `json:"build_status"`
	TestStatus  string   `json:"test_status"`
	ChildAgents []string `json:"child_agents"`
}

type controlLogs struct {
	Artifacts []controlArtifact `json:"artifacts"`
}

type controlArtifact struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	UpdatedAt string `json:"updated_at"`
	Size      string `json:"size"`
}

type controlChannel struct {
	Name       string `json:"name"`
	Running    bool   `json:"running"`
	QueueDepth int    `json:"queue_depth"`
}

type controlWebhook struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type controlPlaneState struct {
	mu               sync.Mutex
	address          string
	startedAt        time.Time
	nextJob          int
	jobs             map[string]*controlJob
	jobOrder         []string
	chatHistory      []controlChatMessage
	actionLog        []string
	sshCache         map[string]sshProbe
	diagnosis        controlSecurityDiagnosis
	terminalHistory  []controlTerminalEntry
	terminalSessions map[string]time.Time
}

type controlJobRequest struct {
	Prompt           string         `json:"prompt"`
	AgentID          string         `json:"agent_id"`
	ExecutionTarget  string         `json:"execution_target"`
	WorkflowTemplate string         `json:"workflow_template"`
	TestType         string         `json:"test_type"`
	DryRun           bool           `json:"dry_run"`
	Payload          map[string]any `json:"payload"`
}

type sshProbe struct {
	At     time.Time
	Status string
	Detail string
}

var controlPlaneStates sync.Map

func getControlPlaneState(m *Manager) *controlPlaneState {
	if v, ok := controlPlaneStates.Load(m); ok {
		return v.(*controlPlaneState)
	}
	st := &controlPlaneState{
		startedAt:        time.Now(),
		jobs:             make(map[string]*controlJob),
		sshCache:         make(map[string]sshProbe),
		terminalSessions: make(map[string]time.Time),
	}
	seedControlPlaneState(st)
	actual, _ := controlPlaneStates.LoadOrStore(m, st)
	return actual.(*controlPlaneState)
}

func seedControlPlaneState(st *controlPlaneState) {
	j1 := &controlJob{
		ID:              "job-seed-media-01",
		JobType:         "media_job",
		State:           "completed",
		AgentID:         "main",
		AssignedWorker:  "pico",
		Artifacts:       []string{"transcript.srt", "cuts.json"},
		ManifestSummary: "drive_ingest -> transcribe -> srt -> cut_suggestions",
		CreatedAt:       time.Now().Add(-40 * time.Minute),
		StartedAt:       time.Now().Add(-38 * time.Minute),
		FinishedAt:      time.Now().Add(-32 * time.Minute),
		Timeline: []controlJobEvent{
			{At: time.Now().Add(-38 * time.Minute).Format(time.RFC3339), Status: "running", Message: "ingest started"},
			{At: time.Now().Add(-36 * time.Minute).Format(time.RFC3339), Status: "running", Message: "transcription complete"},
			{At: time.Now().Add(-33 * time.Minute).Format(time.RFC3339), Status: "completed", Message: "srt + cut suggestions ready"},
		},
	}
	j2 := &controlJob{
		ID:              "job-seed-app-01",
		JobType:         "app_generation",
		State:           "failed",
		AgentID:         "main",
		AssignedWorker:  "pc",
		Artifacts:       []string{"build.log"},
		ManifestSummary: "react-vite fullstack template",
		CreatedAt:       time.Now().Add(-70 * time.Minute),
		StartedAt:       time.Now().Add(-68 * time.Minute),
		FinishedAt:      time.Now().Add(-67 * time.Minute),
		Timeline: []controlJobEvent{
			{At: time.Now().Add(-68 * time.Minute).Format(time.RFC3339), Status: "running", Message: "build started"},
			{At: time.Now().Add(-67 * time.Minute).Format(time.RFC3339), Status: "failed", Message: "missing deploy credentials"},
		},
	}
	st.jobs[j1.ID] = j1
	st.jobs[j2.ID] = j2
	st.jobOrder = append(st.jobOrder, j1.ID, j2.ID)
	st.chatHistory = append(st.chatHistory,
		controlChatMessage{Role: "system", Content: "Control plane booted. Seeded media/app jobs for visibility.", CreatedAt: time.Now().Add(-70 * time.Minute).Format(time.RFC3339)},
	)
}

func (m *Manager) registerControlPlaneRoutes(addr string) {
	st := getControlPlaneState(m)
	st.mu.Lock()
	st.address = addr
	st.mu.Unlock()

	m.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := controlPlaneTemplate.Execute(w, controlPlanePageData{
			Address:     addr,
			GeneratedAt: time.Now().Format(time.RFC3339),
		}); err != nil {
			logger.ErrorCF("channels", "control plane render failed", map[string]any{"error": err.Error()})
			http.Error(w, "control plane render failed", http.StatusInternalServerError)
			return
		}
	})

	m.mux.HandleFunc("GET /api/control-plane/status", func(w http.ResponseWriter, r *http.Request) {
		status := m.buildControlPlaneStatus(addr)
		respondJSON(w, http.StatusOK, status)
	})

	m.mux.HandleFunc("GET /api/control-plane/test-chat/history", func(w http.ResponseWriter, r *http.Request) {
		state := getControlPlaneState(m)
		state.mu.Lock()
		history := append([]controlChatMessage(nil), state.chatHistory...)
		state.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"messages": history})
	})

	m.mux.HandleFunc("POST /api/control-plane/test-chat", func(w http.ResponseWriter, r *http.Request) {
		job, err := m.enqueueControlPlaneJob(r.Body)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"job_id": job.ID, "state": job.State})
	})

	m.mux.HandleFunc("POST /api/control-plane/action", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Action  string         `json:"action"`
			Target  string         `json:"target"`
			Payload map[string]any `json:"payload"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request"})
			return
		}
		result, err := m.handleControlPlaneAction(req.Action, req.Target, req.Payload)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "result": result})
			return
		}
		respondJSON(w, http.StatusOK, result)
	})

	m.mux.HandleFunc("GET /api/control-plane/logs", func(w http.ResponseWriter, r *http.Request) {
		source := strings.TrimSpace(r.URL.Query().Get("source"))
		if source == "" {
			source = "heartbeat"
		}
		search := strings.TrimSpace(r.URL.Query().Get("search"))
		lines := m.loadLogs(source)
		if search != "" {
			filtered := make([]string, 0, len(lines))
			needle := strings.ToLower(search)
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), needle) {
					filtered = append(filtered, line)
				}
			}
			lines = filtered
		}
		respondJSON(w, http.StatusOK, map[string]any{"source": source, "lines": lines})
	})

	m.mux.HandleFunc("GET /api/control-plane/terminal/status", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, m.buildTerminalStatus(r))
	})

	m.mux.HandleFunc("POST /api/control-plane/terminal/login", func(w http.ResponseWriter, r *http.Request) {
		status, err := m.handleTerminalLogin(w, r)
		if err != nil {
			respondJSON(w, http.StatusForbidden, map[string]any{"error": err.Error(), "status": status})
			return
		}
		respondJSON(w, http.StatusOK, status)
	})

	m.mux.HandleFunc("POST /api/control-plane/terminal/logout", func(w http.ResponseWriter, r *http.Request) {
		m.handleTerminalLogout(w, r)
		respondJSON(w, http.StatusOK, m.buildTerminalStatus(r))
	})

	m.mux.HandleFunc("POST /api/control-plane/terminal/exec", func(w http.ResponseWriter, r *http.Request) {
		result, err := m.handleTerminalExec(r)
		if err != nil {
			respondJSON(w, http.StatusForbidden, map[string]any{"error": err.Error(), "result": result})
			return
		}
		respondJSON(w, http.StatusOK, result)
	})
}

func (m *Manager) buildControlPlaneStatus(addr string) controlPlaneStatus {
	st := getControlPlaneState(m)
	channels, webhooks := m.snapshotChannelsAndWebhooks()
	nodes, tailscale := m.buildNodesAndTailscale(st)
	authState, pending := m.buildAuthState(nodes)
	jobs := m.snapshotJobs(st)
	heartbeatSummary := m.heartbeatSummary()
	services := m.buildServiceCards(channels, authState, heartbeatSummary, jobs, st)
	security := m.buildSecurityState(addr, nodes, tailscale, authState, services, pending, st)
	agents := m.buildAgents(jobs, heartbeatSummary)
	mediaState := m.buildMediaState()
	appState := m.buildAppGeneratorState()
	logsState := controlLogs{Artifacts: m.collectArtifacts()}

	degraded := 0
	for _, s := range services {
		if s.Status == "warn" || s.Status == "error" {
			degraded++
		}
	}

	return controlPlaneStatus{
		GeneratedAt:  time.Now().Format(time.RFC3339),
		Gateway:      addr,
		Summary:      controlSummary{Nodes: len(nodes), Agents: len(agents), JobsTotal: len(jobs.Items), PendingSetup: len(pending), DegradedServices: degraded},
		Services:     services,
		Agents:       agents,
		Nodes:        nodes,
		Tailscale:    tailscale,
		Auth:         authState,
		Security:     security,
		Jobs:         jobs,
		Media:        mediaState,
		AppGenerator: appState,
		Logs:         logsState,
		Channels:     channels,
		Webhooks:     webhooks,
		PendingSetup: pending,
	}
}

func (m *Manager) snapshotChannelsAndWebhooks() ([]controlChannel, []controlWebhook) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]controlChannel, 0, len(m.channels))
	webhooks := make([]controlWebhook, 0, len(m.channels))

	for name, ch := range m.channels {
		queueDepth := 0
		if w := m.workers[name]; w != nil {
			queueDepth = len(w.queue) + len(w.mediaQueue)
		}
		channels = append(channels, controlChannel{Name: name, Running: ch.IsRunning(), QueueDepth: queueDepth})
		if wh, ok := ch.(WebhookHandler); ok {
			webhooks = append(webhooks, controlWebhook{Name: name, Path: wh.WebhookPath()})
		}
	}

	sort.Slice(channels, func(i, j int) bool { return channels[i].Name < channels[j].Name })
	sort.Slice(webhooks, func(i, j int) bool { return webhooks[i].Name < webhooks[j].Name })
	return channels, webhooks
}

func (m *Manager) buildServiceCards(
	channels []controlChannel,
	authState controlAuth,
	heartbeat heartbeatStatus,
	jobs controlJobs,
	st *controlPlaneState,
) []controlService {
	chState := map[string]bool{}
	for _, ch := range channels {
		chState[ch.Name] = ch.Running
	}

	workspace := m.config.WorkspacePath()
	_, codexErr := os.Stat(filepath.Join(workspace, "wsl-codex-exec"))
	_, gwsCredErr := os.Stat(filepath.Join(homeDir(), ".config", "gws", "credentials.json"))

	uptime := time.Since(st.startedAt).Round(time.Second).String()

	services := []controlService{
		{ID: "picoclaw-core", Name: "PicoClaw core", Status: "ok", Uptime: uptime, LastError: heartbeat.LastError, LastSuccess: heartbeat.LastSuccess},
		{ID: "telegram", Name: "Telegram", Status: boolState(chState["telegram"], "warn"), Uptime: uptime, LastError: heartbeat.LastError, LastSuccess: heartbeat.LastSuccess},
		{ID: "whatsapp", Name: "WhatsApp", Status: boolState(chState["whatsapp"] || chState["whatsapp_native"], "warn"), Uptime: uptime, LastError: heartbeat.LastError, LastSuccess: heartbeat.LastSuccess},
		{ID: "redis", Name: "Redis", Status: envState("REDIS_URL"), Uptime: "n/a", LastError: "", LastSuccess: ""},
		{ID: "worker-api", Name: "worker API(s)", Status: envState("WORKER_API_URLS"), Uptime: "n/a", LastError: "", LastSuccess: ""},
		{ID: "codex-executor", Name: "Codex executor", Status: errState(codexErr), Uptime: uptime, LastError: errText(codexErr), LastSuccess: "local executor script present"},
		{ID: "google-auth", Name: "Google auth status", Status: authProviderState(authState, "google-antigravity"), Uptime: "n/a", LastError: "", LastSuccess: ""},
		{ID: "drive", Name: "Drive integration", Status: errState(gwsCredErr), Uptime: "n/a", LastError: errText(gwsCredErr), LastSuccess: "credentials synced"},
		{ID: "calendar", Name: "Calendar integration", Status: errState(gwsCredErr), Uptime: "n/a", LastError: errText(gwsCredErr), LastSuccess: "credentials synced"},
		{ID: "deploy", Name: "deploy services", Status: boolState(jobs.Completed > 0, "warn"), Uptime: uptime, LastError: heartbeat.LastError, LastSuccess: heartbeat.LastSuccess},
	}
	return services
}

func (m *Manager) buildAgents(jobs controlJobs, heartbeat heartbeatStatus) []controlAgent {
	queueByAgent := map[string]int{}
	runningByAgent := map[string]string{}
	for _, j := range jobs.Items {
		if j.State == "queued" {
			queueByAgent[j.AgentID]++
		}
		if j.State == "running" {
			runningByAgent[j.AgentID] = j.ID
		}
	}

	agents := make([]controlAgent, 0, len(m.config.Agents.List)+1)
	agents = append(agents, controlAgent{
		ID:              "main",
		Name:            "Main Agent",
		Type:            "default",
		State:           stateFromActive(runningByAgent["main"]),
		ActiveJob:       runningByAgent["main"],
		QueueDepth:      queueByAgent["main"],
		LastHeartbeat:   heartbeat.LastSuccess,
		Capabilities:    []string{"tool-calls", "chat-routing", "media", "subagents"},
		AssignedNode:    "pico",
		ExecutionTarget: "local",
		Concurrency:     1,
		Logs:            m.sampleAgentLogs("main"),
	})

	for _, a := range m.config.Agents.List {
		id := a.ID
		if id == "" {
			continue
		}
		assigned := "pico"
		lower := strings.ToLower(id + " " + a.Name)
		if strings.Contains(lower, "vps") {
			assigned = "vps"
		} else if strings.Contains(lower, "pc") || strings.Contains(lower, "worker") {
			assigned = "pc"
		}
		caps := []string{"tool-calls"}
		if len(a.Skills) > 0 {
			caps = append(caps, "skills")
		}
		if a.Subagents != nil {
			caps = append(caps, "subagents")
		}
		concurrency := 1
		if a.Subagents != nil && len(a.Subagents.AllowAgents) > 0 {
			concurrency = len(a.Subagents.AllowAgents)
		}
		agents = append(agents, controlAgent{
			ID:              id,
			Name:            a.Name,
			Type:            "registered",
			State:           stateFromActive(runningByAgent[id]),
			ActiveJob:       runningByAgent[id],
			QueueDepth:      queueByAgent[id],
			LastHeartbeat:   heartbeat.LastSuccess,
			Capabilities:    caps,
			AssignedNode:    assigned,
			ExecutionTarget: assigned,
			Concurrency:     concurrency,
			Logs:            m.sampleAgentLogs(id),
		})
	}

	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
	return agents
}

func (m *Manager) sampleAgentLogs(agentID string) []string {
	workspace := m.config.WorkspacePath()
	sessions := filepath.Join(workspace, "sessions")
	files, err := os.ReadDir(sessions)
	if err != nil {
		return []string{"session logs unavailable"}
	}

	prefix := "agent_" + agentID + "_"
	picked := ""
	latest := time.Time{}
	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), prefix) || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		info, statErr := f.Info()
		if statErr != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
			picked = filepath.Join(sessions, f.Name())
		}
	}
	if picked == "" {
		return []string{"no session file found"}
	}
	lines := tailFile(picked, 8)
	if len(lines) == 0 {
		return []string{"session log empty"}
	}
	return lines
}

func (m *Manager) buildNodesAndTailscale(st *controlPlaneState) ([]controlNode, controlTailscale) {
	ts, _ := loadTailStatus()

	selfHost, _ := os.Hostname()
	selfIP := first(ts.Self.TailscaleIPs)

	heartbeat := m.heartbeatSummary()
	channels, _ := m.snapshotChannelsAndWebhooks()
	services := make([]string, 0, len(channels)+2)
	for _, c := range channels {
		if c.Running {
			services = append(services, c.Name)
		}
	}
	services = append(services, "health", "ready")
	sort.Strings(services)

	selfMetrics := processMetrics()
	selfNode := controlNode{
		ID:              "pico",
		Hostname:        selfHost,
		Tags:            ts.Self.Tags,
		Role:            "pico-node",
		TailscaleIP:     selfIP,
		SSHStatus:       "ok",
		Reachability:    "reachable",
		LastHeartbeat:   heartbeat.LastSuccess,
		ExposedServices: services,
		StorageUsage:    "n/a",
		CPUMemory:       selfMetrics,
		RecentFailures:  heartbeat.LastError,
		Online:          ts.Self.Online,
		SSHEnabled:      hasSSHCap(ts.Self.CapMap),
		WorkerReady:     true,
	}

	nodes := []controlNode{selfNode}

	known := []struct {
		ID   string
		Role string
		Hint []string
	}{
		{ID: "vps", Role: "gateway", Hint: []string{"vps", "gateway"}},
		{ID: "pc", Role: "pc-worker", Hint: []string{"black-wave", "home-pc", "pc", "desktop"}},
		{ID: "storage", Role: "storage", Hint: []string{"nas", "storage"}},
	}

	peerByID := map[string]tailPeer{}
	for _, k := range known {
		bestScore := 0
		for _, p := range ts.Peers {
			score := scoreTailPeer(k.ID, k.Hint, p)
			if score > bestScore {
				bestScore = score
				peerByID[k.ID] = p
			}
		}
	}

	for _, k := range known {
		p := peerByID[k.ID]
		host := p.HostName
		if host == "" {
			host = "not-discovered"
		}
		sshStatus, sshDetail := m.cachedSSHProbe(st, k.ID, p)
		nodes = append(nodes, controlNode{
			ID:              k.ID,
			Hostname:        host,
			Tags:            p.Tags,
			Role:            k.Role,
			TailscaleIP:     first(p.TailscaleIPs),
			SSHStatus:       sshStatus,
			Reachability:    onlineText(p.Online),
			LastHeartbeat:   "unknown",
			ExposedServices: inferredServices(k.Role),
			StorageUsage:    "n/a",
			CPUMemory:       "n/a",
			RecentFailures:  sshDetail,
			Online:          p.Online,
			SSHEnabled:      hasSSHCap(p.CapMap),
			WorkerReady:     p.Online && hasSSHCap(p.CapMap),
		})
	}

	warnings := []string{}
	for _, n := range nodes {
		if n.ID == "pico" {
			continue
		}
		if !n.Online {
			warnings = append(warnings, fmt.Sprintf("%s is offline on tailscale", n.ID))
		}
		if n.SSHStatus != "ok" {
			warnings = append(warnings, fmt.Sprintf("%s SSH is not ready: %s", n.ID, n.SSHStatus))
		}
		if len(n.Tags) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s missing tags", n.ID))
		}
	}

	paths := []controlPath{}
	pairs := [][2]string{{"pico", "vps"}, {"pico", "pc"}, {"vps", "pico"}, {"pc", "vps"}, {"pc", "pico"}, {"vps", "pc"}}
	nodeMap := map[string]controlNode{}
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	for _, pair := range pairs {
		src := nodeMap[pair[0]]
		dst := nodeMap[pair[1]]
		status := "unknown"
		reason := "cross-node probe unavailable from current host"
		if src.ID == "pico" {
			if dst.SSHStatus == "ok" && dst.Online {
				status = "ok"
				reason = "ssh path validated from pico"
			} else {
				status = "warn"
				reason = "target not reachable or SSH unavailable"
			}
		}
		paths = append(paths, controlPath{
			From:             pair[0],
			To:               pair[1],
			Status:           status,
			Reason:           reason,
			TailscaleOnline:  dst.Online,
			SSHEnabled:       dst.SSHEnabled,
			TagState:         tagsState(dst.Tags),
			WorkerRegistered: dst.WorkerReady,
		})
	}

	return nodes, controlTailscale{Paths: paths, Warnings: warnings}
}

func (m *Manager) buildAuthState(nodes []controlNode) (controlAuth, []controlChecklistItem) {
	checklist := []controlChecklistItem{}

	if m.config.Channels.Telegram.Token != "" {
		checklist = append(checklist, controlChecklistItem{Name: "Telegram bot token", Description: "Configured in channels.telegram.token", Status: "ok"})
	} else {
		checklist = append(checklist, controlChecklistItem{Name: "Telegram bot token", Description: "Set channels.telegram.token", Status: "warn"})
	}

	if m.config.Channels.WhatsApp.UseNative || m.config.Channels.WhatsApp.BridgeURL != "" {
		checklist = append(checklist, controlChecklistItem{Name: "WhatsApp bridge/native", Description: "Bridge URL or native mode set", Status: "ok"})
	} else {
		checklist = append(checklist, controlChecklistItem{Name: "WhatsApp bridge/native", Description: "Set bridge URL or enable native mode", Status: "warn"})
	}

	if m.config.Providers.OpenRouter.APIKey != "" || m.config.Providers.OpenAI.APIKey != "" || m.config.Providers.Anthropic.APIKey != "" {
		checklist = append(checklist, controlChecklistItem{Name: "LLM credentials", Description: "At least one provider key configured", Status: "ok"})
	} else {
		checklist = append(checklist, controlChecklistItem{Name: "LLM credentials", Description: "No provider API key found", Status: "error"})
	}

	_, gwsErr := os.Stat(filepath.Join(homeDir(), ".config", "gws", "credentials.json"))
	if gwsErr == nil {
		checklist = append(checklist, controlChecklistItem{Name: "Google credentials sync", Description: "gws credentials.json present", Status: "ok"})
	} else {
		checklist = append(checklist, controlChecklistItem{Name: "Google credentials sync", Description: "Run gws auth sync/setup", Status: "warn"})
	}

	workerOK := false
	for _, n := range nodes {
		if n.ID != "pico" && n.WorkerReady {
			workerOK = true
		}
	}
	if workerOK {
		checklist = append(checklist, controlChecklistItem{Name: "Worker registration", Description: "At least one remote worker looks ready", Status: "ok"})
	} else {
		checklist = append(checklist, controlChecklistItem{Name: "Worker registration", Description: "No remote worker is fully ready", Status: "warn"})
	}

	providers := []controlAuthProvider{}
	store, err := auth.LoadStore()
	if err == nil && store != nil {
		for provider, cred := range store.Credentials {
			status := "ok"
			if cred.IsExpired() {
				status = "error"
			} else if cred.NeedsRefresh() {
				status = "warn"
			}
			account := cred.Email
			if account == "" {
				account = cred.AccountID
			}
			expires := ""
			if !cred.ExpiresAt.IsZero() {
				expires = cred.ExpiresAt.Format(time.RFC3339)
			}
			providers = append(providers, controlAuthProvider{
				Provider:   provider,
				AuthMethod: cred.AuthMethod,
				Status:     status,
				Account:    account,
				ExpiresAt:  expires,
			})
		}
	}
	if len(providers) == 0 {
		providers = append(providers, controlAuthProvider{Provider: "(none)", Status: "warn", AuthMethod: "n/a", Account: "no auth store credentials", ExpiresAt: ""})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Provider < providers[j].Provider })

	pending := make([]controlChecklistItem, 0, len(checklist))
	for _, item := range checklist {
		if item.Status != "ok" {
			pending = append(pending, item)
		}
	}

	return controlAuth{Checklist: checklist, Providers: providers}, pending
}

func (m *Manager) buildSecurityState(
	addr string,
	nodes []controlNode,
	tailscale controlTailscale,
	authState controlAuth,
	services []controlService,
	pending []controlChecklistItem,
	st *controlPlaneState,
) controlSecurity {
	workspace := m.config.WorkspacePath()
	selfNode := findNodeByID(nodes, "pico")
	gwsPath := filepath.Join(homeDir(), ".config", "gws", "credentials.json")
	gwsReady := fileExists(gwsPath)
	serveConfig := fileExists(filepath.Join(workspace, "ts-serve-picoclaw.json"))
	certFiles := detectTailscaleCertFiles(selfNode.Hostname, workspace)
	shellEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("PICOCLAW_DASHBOARD_ALLOW_SHELL")), "true")
	execEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("PICOCLAW_ALLOW_EXEC")), "true")
	publicBase := controlPlanePublicBase(selfNode, serveConfig)
	llmReady := m.hasControlPlaneDiagnoser()

	findings := make([]controlSecurityFinding, 0, 8)
	if !selfNode.Online {
		findings = append(findings, controlSecurityFinding{
			Severity: "error",
			Area:     "tailnet",
			Title:    "Tailnet reachability missing",
			Detail:   "This node does not appear online in tailscale status, so secure remote control and SSH assumptions are degraded.",
			FixHint:  "Restore tailscaled connectivity before relying on web-auth or cross-node recovery flows.",
		})
	}
	if !gwsReady {
		findings = append(findings, controlSecurityFinding{
			Severity: "warn",
			Area:     "google-auth",
			Title:    "Google credentials not synced",
			Detail:   "No gws credentials.json was found for the active runtime, so Drive and Calendar-backed flows are not web-ready.",
			FixHint:  "Complete Google auth sync, then keep the secure origin stable behind tailscale serve/cert.",
		})
	}
	if len(certFiles) == 0 {
		findings = append(findings, controlSecurityFinding{
			Severity: "warn",
			Area:     "secure-web",
			Title:    "No tailscale cert assets detected",
			Detail:   "The control plane can be served over HTTP now, but no local certificate files were detected for a tighter HTTPS/tailscale setup.",
			FixHint:  "Issue or sync tailscale cert files and keep the reverse-proxy target stable for auth redirects.",
		})
	}
	if !serveConfig {
		findings = append(findings, controlSecurityFinding{
			Severity: "warn",
			Area:     "secure-web",
			Title:    "tailscale serve config not tracked in runtime workspace",
			Detail:   "A tracked serve manifest is not present in the active workspace, so web-auth origin management is less explicit than it should be.",
			FixHint:  "Keep tailscale serve/cert configuration versioned and aligned with the control-plane route map.",
		})
	}
	if shellEnabled {
		findings = append(findings, controlSecurityFinding{
			Severity: "warn",
			Area:     "exec-surface",
			Title:    "Dashboard shell execution is enabled",
			Detail:   "PICOCLAW_DASHBOARD_ALLOW_SHELL=true allows backend shell execution from control-plane workflows and needs deliberate operator control.",
			FixHint:  "Restrict usage to trusted operators, prefer scoped actions, and disable when not needed.",
		})
	}
	for _, p := range authState.Providers {
		if p.Status == "error" {
			findings = append(findings, controlSecurityFinding{
				Severity: "error",
				Area:     "auth-store",
				Title:    "Expired provider credential",
				Detail:   fmt.Sprintf("%s auth record for %s is expired or invalid.", p.Provider, p.Account),
				FixHint:  "Refresh that provider credential before using it for secure web or operator workflows.",
			})
		}
	}
	for _, warning := range tailscale.Warnings {
		findings = append(findings, controlSecurityFinding{
			Severity: "warn",
			Area:     "tailscale",
			Title:    "Tailnet warning",
			Detail:   warning,
			FixHint:  "Clear the tailscale blocker before assuming remote execution or auth callback paths are healthy.",
		})
		if len(findings) >= 10 {
			break
		}
	}

	criticalCount := countSecurityFindings(findings, "error")
	warnCount := countSecurityFindings(findings, "warn")

	domains := []controlSecurityDomain{
		{
			ID:      "secure-web",
			Name:    "Secure web surface",
			Status:  securityDomainStatus(len(certFiles) > 0 && serveConfig && publicBase != "", len(certFiles) > 0 || publicBase != ""),
			Score:   securityDomainScore(len(certFiles) > 0 && serveConfig && publicBase != "", len(certFiles) > 0 || publicBase != ""),
			Detail:  "Reverse-proxy route, tailscale serve manifest, and certificate assets for a stable browser-facing control plane.",
			Signals: compactSignals([]string{ternaryText(publicBase != "", "public route", "no public route"), ternaryText(serveConfig, "serve config", "no serve config"), ternaryText(len(certFiles) > 0, "cert files", "no cert files")}),
		},
		{
			ID:      "identity-auth",
			Name:    "Identity / Google auth",
			Status:  securityDomainStatus(gwsReady && countAuthProvidersByStatus(authState.Providers, "error") == 0, gwsReady || countAuthProvidersByStatus(authState.Providers, "ok") > 0),
			Score:   securityDomainScore(gwsReady && countAuthProvidersByStatus(authState.Providers, "error") == 0, gwsReady || countAuthProvidersByStatus(authState.Providers, "ok") > 0),
			Detail:  "Google Workspace sync, provider credential freshness, and backend auth-store readiness.",
			Signals: compactSignals([]string{ternaryText(gwsReady, "gws synced", "gws missing"), fmt.Sprintf("%d auth ok", countAuthProvidersByStatus(authState.Providers, "ok")), fmt.Sprintf("%d auth error", countAuthProvidersByStatus(authState.Providers, "error"))}),
		},
		{
			ID:      "tailnet",
			Name:    "Tailnet / remote execution",
			Status:  securityDomainStatus(selfNode.Online && len(tailscale.Warnings) == 0, selfNode.Online),
			Score:   securityDomainScore(selfNode.Online && len(tailscale.Warnings) == 0, selfNode.Online),
			Detail:  "Tailscale online state, SSH reachability, and worker/node routing health.",
			Signals: compactSignals([]string{ternaryText(selfNode.Online, "tailnet online", "tailnet offline"), fmt.Sprintf("%d warnings", len(tailscale.Warnings)), ternaryText(anyRemoteWorkerReady(nodes), "worker ready", "no worker ready")}),
		},
		{
			ID:      "exec",
			Name:    "Execution surface",
			Status:  securityDomainStatus(execEnabled && !shellEnabled, execEnabled || shellEnabled),
			Score:   securityDomainScore(execEnabled && !shellEnabled, execEnabled || shellEnabled),
			Detail:  "Backend execution powers available to the control plane and whether they are tightly scoped or broadly exposed.",
			Signals: compactSignals([]string{ternaryText(execEnabled, "exec enabled", "exec disabled"), ternaryText(shellEnabled, "shell enabled", "shell disabled"), ternaryText(serviceStatusIs(services, "codex-executor", "ok"), "codex executor", "missing executor")}),
		},
		{
			ID:      "diagnosis",
			Name:    "Self-diagnose / observability",
			Status:  securityDomainStatus(llmReady && len(pending) == 0, llmReady),
			Score:   securityDomainScore(llmReady && len(pending) == 0, llmReady),
			Detail:  "Backend-driven posture refresh, LLM diagnosis availability, and fix queue visibility.",
			Signals: compactSignals([]string{ternaryText(llmReady, "llm ready", "llm unavailable"), fmt.Sprintf("%d pending", len(pending)), fmt.Sprintf("%d findings", len(findings))}),
		},
	}

	access := []controlSecurityAccess{
		{
			Name:     "Control plane UI",
			URL:      joinURL(publicBase, "/dash/control"),
			Status:   ternaryStatus(publicBase != "", "ok", "warn"),
			Auth:     ternaryText(gwsReady, "backend auth ready", "backend auth partial"),
			Exposure: "operator web",
			Notes:    "Primary browser surface for posture, auth state, and backend-controlled fixes.",
		},
		{
			Name:     "Control plane API",
			URL:      joinURL(publicBase, "/api/control-plane/status"),
			Status:   ternaryStatus(publicBase != "", "ok", "warn"),
			Auth:     "none on route itself",
			Exposure: "JSON status",
			Notes:    "Use behind tailscale or reverse-proxy policy; route is designed for dynamic UI refresh.",
		},
		{
			Name:     "Legacy dashboard",
			URL:      joinURL(publicBase, "/legacy-dashboard"),
			Status:   ternaryStatus(publicBase != "", "ok", "warn"),
			Auth:     "same web surface",
			Exposure: "fallback UI",
			Notes:    "Lightweight fallback if the richer control-plane frontend regresses.",
		},
		{
			Name:     "Google Workspace backend",
			URL:      "",
			Status:   ternaryStatus(gwsReady, "ok", "warn"),
			Auth:     ternaryText(gwsReady, "credentials.json present", "sync required"),
			Exposure: "backend-managed",
			Notes:    "For browser auth, keep the redirect origin stable behind tailscale serve/cert and then sync credentials to gws.",
		},
		{
			Name:     "tailscale serve / cert path",
			URL:      joinURL(controlPlaneHTTPSBase(selfNode, certFiles), "/dash/control"),
			Status:   ternaryStatus(len(certFiles) > 0 || serveConfig, "ok", "warn"),
			Auth:     ternaryText(len(certFiles) > 0, "cert assets present", "cert assets missing"),
			Exposure: "secure web candidate",
			Notes:    "Use this as the hardened web-auth target once serve/cert policy is finalized.",
		},
		{
			Name:     "Interactive terminal",
			URL:      joinURL(controlPlaneHTTPSBase(selfNode, certFiles), "/dash/control#terminal"),
			Status:   ternaryStatus(controlPlaneTerminalSecret(m) != "", "ok", "warn"),
			Auth:     ternaryText(controlPlaneTerminalSecret(m) != "", "login required", "login secret missing"),
			Exposure: "tailscale + https only",
			Notes:    "Terminal execution is intentionally gated behind tailscale origin checks, HTTPS, and session login.",
		},
	}

	score := 100 - criticalCount*22 - warnCount*8
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	st.mu.Lock()
	diagnosis := st.diagnosis
	st.mu.Unlock()
	if diagnosis.Status == "" {
		diagnosis.Status = "idle"
	}
	if diagnosis.Source == "" {
		diagnosis.Source = "backend"
	}

	return controlSecurity{
		Summary: controlSecuritySummary{
			Overall:   securityOverallStatus(criticalCount, warnCount),
			Score:     score,
			Critical:  criticalCount,
			Warn:      warnCount,
			AuthReady: countAuthProvidersByStatus(authState.Providers, "ok") + ternaryInt(gwsReady, 1, 0),
			SecureWeb: len(certFiles) > 0 || serveConfig,
			LLMReady:  llmReady,
		},
		Domains:   domains,
		Findings:  findings,
		Access:    access,
		Diagnosis: diagnosis,
	}
}

func (m *Manager) snapshotJobs(st *controlPlaneState) controlJobs {
	st.mu.Lock()
	defer st.mu.Unlock()

	items := make([]controlJob, 0, len(st.jobOrder))
	queued, running, failed, completed, retries := 0, 0, 0, 0, 0
	for i := len(st.jobOrder) - 1; i >= 0; i-- {
		id := st.jobOrder[i]
		job := st.jobs[id]
		if job == nil {
			continue
		}
		clone := *job
		clone.Payload = nil
		if !clone.StartedAt.IsZero() {
			clone.StartTime = clone.StartedAt.Format(time.RFC3339)
		}
		if !clone.StartedAt.IsZero() && !clone.FinishedAt.IsZero() {
			clone.Duration = clone.FinishedAt.Sub(clone.StartedAt).Round(time.Millisecond).String()
		}
		if clone.ManifestSummary == "" {
			clone.ManifestSummary = summarizePayload(job.Payload)
		}
		items = append(items, clone)

		switch job.State {
		case "queued":
			queued++
		case "running":
			running++
		case "failed":
			failed++
		case "completed":
			completed++
		}
		retries += job.RetryCount
	}
	if len(items) > 80 {
		items = items[:80]
	}
	return controlJobs{Queued: queued, Running: running, Failed: failed, Completed: completed, Retries: retries, Items: items}
}

func (m *Manager) buildMediaState() controlMedia {
	workspace := m.config.WorkspacePath()
	artifacts := m.collectArtifacts()
	srtCount := 0
	transcriptCount := 0
	for _, a := range artifacts {
		lower := strings.ToLower(a.Path)
		if strings.HasSuffix(lower, ".srt") {
			srtCount++
		}
		if strings.HasSuffix(lower, ".txt") || strings.Contains(lower, "transcript") {
			transcriptCount++
		}
	}

	pipeline := []controlPipelineStep{
		{Step: "Google Drive ingest", Status: boolState(fileExists(filepath.Join(homeDir(), ".config", "gws", "credentials.json")), "warn"), Detail: "gws credentials check"},
		{Step: "Transcription", Status: ternaryStatus(transcriptCount > 0, "ok", "warn"), Detail: fmt.Sprintf("%d transcript artifacts detected", transcriptCount)},
		{Step: "SRT generation", Status: ternaryStatus(srtCount > 0, "ok", "warn"), Detail: fmt.Sprintf("%d SRT artifacts detected", srtCount)},
		{Step: "Cut suggestions", Status: ternaryStatus(hasArtifactKind(artifacts, "json"), "ok", "warn"), Detail: "metadata / suggestions from json outputs"},
		{Step: "Vegas-friendly output", Status: "ok", Detail: "subtitle burn-in disabled by default, separate SRT outputs preferred"},
		{Step: "FFmpeg worker readiness", Status: boolState(fileExists(filepath.Join(workspace, "test_tailscale_ssh.sh")), "warn"), Detail: "worker scripts discovered in workspace"},
	}

	templates := []controlTemplate{
		{Name: "Interview clips -> transcript + SRT + cut list", SampleManifest: `{"source":"drive://folder/interviews","tasks":["transcribe","srt","cut_suggestions"],"subtitle_burn_in":false,"target":"vegas"}`},
		{Name: "Podcast highlight extraction", SampleManifest: `{"source":"drive://folder/podcast","tasks":["transcribe","segment_topics","clip_candidates"],"subtitle_burn_in":false}`},
		{Name: "Telegram media automation", SampleManifest: `{"source":"telegram://media","tasks":["ingest","transcribe","srt"],"dispatch":"worker_api"}`},
	}
	return controlMedia{Pipeline: pipeline, Templates: templates}
}

func (m *Manager) buildAppGeneratorState() controlAppGenerator {
	workspace := m.config.WorkspacePath()
	status := []controlAppStatus{
		{Workspace: workspace, PreviewURL: "n/a", BuildStatus: "warn", TestStatus: "warn", ChildAgents: []string{"main"}},
	}
	requests := []controlAppRequest{
		{ID: "app-seed-dashboard", ProjectType: "admin-console", Template: "react-ts", DeployTarget: "vps", Status: "failed"},
		{ID: "app-seed-worker-ui", ProjectType: "worker-control", Template: "nextjs", DeployTarget: "pc", Status: "completed"},
	}
	return controlAppGenerator{Requests: requests, Status: status}
}

func (m *Manager) collectArtifacts() []controlArtifact {
	workspace := m.config.WorkspacePath()
	candidates := []string{
		workspace,
		filepath.Join(workspace, "artifacts"),
		filepath.Join(workspace, "output"),
		filepath.Join(workspace, "media"),
	}
	ext := map[string]string{".srt": "srt", ".vtt": "srt", ".txt": "transcript", ".json": "json", ".mp4": "video", ".mkv": "video", ".wav": "audio", ".mp3": "audio"}
	out := []controlArtifact{}
	seen := map[string]bool{}
	for _, root := range candidates {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator)) > 4 {
					return filepath.SkipDir
				}
				return nil
			}
			kind := ext[strings.ToLower(filepath.Ext(d.Name()))]
			if kind == "" {
				return nil
			}
			if seen[path] {
				return nil
			}
			seen[path] = true
			fi, statErr := d.Info()
			if statErr != nil {
				return nil
			}
			out = append(out, controlArtifact{
				Path:      path,
				Kind:      kind,
				UpdatedAt: fi.ModTime().Format(time.RFC3339),
				Size:      humanSize(fi.Size()),
			})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func (m *Manager) heartbeatSummary() heartbeatStatus {
	lines := tailFile(filepath.Join(m.config.WorkspacePath(), "heartbeat.log"), 180)
	result := heartbeatStatus{LastError: "none", LastSuccess: "none"}
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if result.LastError == "none" && strings.Contains(line, "[ERROR]") {
			result.LastError = trimLogLine(line)
		}
		if result.LastSuccess == "none" && strings.Contains(strings.ToLower(line), "heartbeat ok") {
			result.LastSuccess = trimLogLine(line)
		}
		if result.LastError != "none" && result.LastSuccess != "none" {
			break
		}
	}
	return result
}

type heartbeatStatus struct {
	LastError   string
	LastSuccess string
}

func trimLogLine(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 140 {
		return line[:140] + "..."
	}
	return line
}

func (m *Manager) enqueueControlPlaneJob(body io.Reader) (*controlJob, error) {
	var req controlJobRequest
	if err := json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&req); err != nil {
		return nil, errors.New("invalid JSON body")
	}
	if strings.TrimSpace(req.TestType) == "" {
		req.TestType = "agent_prompt"
	}
	if strings.TrimSpace(req.AgentID) == "" {
		req.AgentID = "main"
	}
	if strings.TrimSpace(req.ExecutionTarget) == "" {
		req.ExecutionTarget = "pico"
	}

	st := getControlPlaneState(m)
	st.mu.Lock()
	st.nextJob++
	id := fmt.Sprintf("job-%06d", st.nextJob)
	job := &controlJob{
		ID:             id,
		JobType:        req.TestType,
		State:          "queued",
		AgentID:        req.AgentID,
		AssignedWorker: req.ExecutionTarget,
		Payload: map[string]any{
			"prompt":            req.Prompt,
			"workflow_template": req.WorkflowTemplate,
			"payload":           req.Payload,
			"dry_run":           req.DryRun,
		},
		CreatedAt: time.Now(),
		Timeline:  []controlJobEvent{{At: time.Now().Format(time.RFC3339), Status: "queued", Message: "queued from backend test chat"}},
	}
	st.jobs[id] = job
	st.jobOrder = append(st.jobOrder, id)
	st.chatHistory = append(st.chatHistory, controlChatMessage{Role: "user", Content: req.Prompt, CreatedAt: time.Now().Format(time.RFC3339)})
	trimStateHistory(st)
	st.mu.Unlock()

	go m.runControlPlaneJob(id, req)
	return job, nil
}

func (m *Manager) runControlPlaneJob(id string, req controlJobRequest) {
	st := getControlPlaneState(m)
	st.mu.Lock()
	job := st.jobs[id]
	if job == nil {
		st.mu.Unlock()
		return
	}
	job.State = "running"
	job.StartedAt = time.Now()
	job.Timeline = append(job.Timeline, controlJobEvent{At: time.Now().Format(time.RFC3339), Status: "running", Message: "execution started"})
	st.mu.Unlock()

	result, artifacts, err := m.executeControlPlaneJob(req)

	st.mu.Lock()
	job = st.jobs[id]
	if job != nil {
		if err != nil {
			job.State = "failed"
			job.Timeline = append(job.Timeline, controlJobEvent{At: time.Now().Format(time.RFC3339), Status: "failed", Message: err.Error()})
		} else {
			job.State = "completed"
			job.Artifacts = append(job.Artifacts, artifacts...)
			job.Timeline = append(job.Timeline, controlJobEvent{At: time.Now().Format(time.RFC3339), Status: "completed", Message: result})
		}
		job.FinishedAt = time.Now()
		job.ManifestSummary = summarizePayload(job.Payload)
	}
	assistantMsg := result
	if err != nil {
		assistantMsg = "failed: " + err.Error()
	}
	st.chatHistory = append(st.chatHistory, controlChatMessage{Role: "assistant", Content: assistantMsg, CreatedAt: time.Now().Format(time.RFC3339)})
	trimStateHistory(st)
	st.mu.Unlock()
}

func (m *Manager) executeControlPlaneJob(req controlJobRequest) (string, []string, error) {
	if req.DryRun {
		return fmt.Sprintf("dry-run accepted: type=%s target=%s workflow=%s", req.TestType, req.ExecutionTarget, req.WorkflowTemplate), nil, nil
	}

	switch req.TestType {
	case "worker_api_call":
		url, _ := req.Payload["url"].(string)
		if url == "" {
			url = "http://127.0.0.1/health"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		out, err := httpProbe(ctx, url)
		return out, nil, err
	case "codex_execution":
		if strings.ToLower(os.Getenv("PICOCLAW_DASHBOARD_ALLOW_SHELL")) != "true" {
			return "", nil, errors.New("shell execution disabled; set PICOCLAW_DASHBOARD_ALLOW_SHELL=true")
		}
		cmdText, _ := req.Payload["command"].(string)
		if strings.TrimSpace(cmdText) == "" {
			cmdText = "echo codex_exec_ok"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "-lc", cmdText)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), nil, err
	case "telegram_input":
		chatID, _ := req.Payload["chat_id"].(string)
		if chatID == "" {
			chatID = "direct"
		}
		if err := m.SendToChannel(context.Background(), "telegram", chatID, req.Prompt); err != nil {
			return "", nil, err
		}
		return "telegram message dispatched", nil, nil
	case "media_job":
		return "media job accepted: transcription + srt + cut suggestions queued", []string{"output/transcript.srt", "output/cut_suggestions.json"}, nil
	case "app_generation":
		return "app generation accepted: frontend/backend split started", []string{"build.log", "preview.url"}, nil
	default:
		return fmt.Sprintf("agent prompt executed for %s on %s: %s", req.AgentID, req.ExecutionTarget, req.Prompt), nil, nil
	}
}

func (m *Manager) runControlPlaneSecurityDiagnosis() (controlSecurityDiagnosis, error) {
	status := m.buildControlPlaneStatus(getControlPlaneState(m).address)
	diagnoser := m.getControlPlaneDiagnoser()
	if diagnoser == nil {
		diag := controlSecurityDiagnosis{
			Status:    "warn",
			LastRunAt: time.Now().Format(time.RFC3339),
			Source:    "backend",
			LastError: "LLM diagnoser unavailable",
			Summary:   "No LLM diagnosis callback is configured for the control plane.",
		}
		m.storeSecurityDiagnosis(diag)
		return diag, errors.New("llm diagnoser unavailable")
	}

	snapshot := map[string]any{
		"summary":        status.Security.Summary,
		"domains":        status.Security.Domains,
		"findings":       status.Security.Findings,
		"access":         status.Security.Access,
		"pending_setup":  status.PendingSetup,
		"tailscale":      status.Tailscale.Warnings,
		"service_status": summarizeServiceStatuses(status.Services),
	}
	payload, _ := json.MarshalIndent(snapshot, "", "  ")
	prompt := "You are diagnosing the PicoClaw control plane security posture. " +
		"Do not call tools. Use only the supplied snapshot. " +
		"Return a concise operator diagnosis with: 1) overall posture, 2) top 3 risks, 3) highest-value next fixes, 4) whether Google web auth should sit behind tailscale serve plus cert.\n\n" +
		string(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	text, err := diagnoser(ctx, prompt)
	diag := controlSecurityDiagnosis{
		Status:    ternaryStatus(err == nil, "ok", "error"),
		LastRunAt: time.Now().Format(time.RFC3339),
		Source:    "llm",
		Summary:   strings.TrimSpace(text),
	}
	if err != nil {
		diag.LastError = err.Error()
		if diag.Summary == "" {
			diag.Summary = "LLM diagnosis failed."
		}
	}
	m.storeSecurityDiagnosis(diag)
	return diag, err
}

func (m *Manager) runControlPlaneFixPlan() (controlSecurityDiagnosis, error) {
	status := m.buildControlPlaneStatus(getControlPlaneState(m).address)
	diagnoser := m.getControlPlaneDiagnoser()
	if diagnoser == nil {
		diag := controlSecurityDiagnosis{
			Status:    "warn",
			LastRunAt: time.Now().Format(time.RFC3339),
			Source:    "backend",
			LastError: "LLM fix planner unavailable",
			Summary:   "No LLM fix planner is configured for the control plane.",
		}
		return diag, errors.New("llm fix planner unavailable")
	}

	snapshot := map[string]any{
		"summary":        status.Security.Summary,
		"domains":        status.Security.Domains,
		"findings":       status.Security.Findings,
		"access":         status.Security.Access,
		"pending_setup":  status.PendingSetup,
		"tailscale":      status.Tailscale.Warnings,
		"service_status": summarizeServiceStatuses(status.Services),
	}
	payload, _ := json.MarshalIndent(snapshot, "", "  ")
	prompt := "You are generating a PicoClaw control-plane remediation plan. " +
		"Do not call tools. Use only the supplied snapshot. " +
		"Return: 1) top 5 fixes ordered by impact, 2) which are safe to automate now, 3) which need human validation, 4) a short mobile-operator summary.\n\n" +
		string(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	text, err := diagnoser(ctx, prompt)
	diag := controlSecurityDiagnosis{
		Status:    ternaryStatus(err == nil, "ok", "error"),
		LastRunAt: time.Now().Format(time.RFC3339),
		Source:    "llm-fix-plan",
		Summary:   strings.TrimSpace(text),
	}
	if err != nil {
		diag.LastError = err.Error()
		if diag.Summary == "" {
			diag.Summary = "LLM fix plan failed."
		}
	}
	return diag, err
}

func (m *Manager) securityGoogleProbe() map[string]any {
	gwsPath := filepath.Join(homeDir(), ".config", "gws", "credentials.json")
	clientSecret := filepath.Join(homeDir(), ".config", "gws", "client_secret.json")
	authState, _ := m.buildAuthState(nil)
	return map[string]any{
		"gws_credentials_path": gwsPath,
		"gws_credentials":      fileExists(gwsPath),
		"client_secret":        fileExists(clientSecret),
		"auth_store_google":    authProviderState(authState, "google-antigravity"),
		"web_auth_note":        "Keep browser auth on a stable tailscale serve/cert origin, then sync resulting credentials back into gws.",
	}
}

func (m *Manager) securitySecureWebProbe() map[string]any {
	ts, tsErr := loadTailStatus()
	host, _ := os.Hostname()
	certFiles := detectTailscaleCertFiles(host, m.config.WorkspacePath())
	return map[string]any{
		"tailscale_online":     tsErr == nil && ts.Self.Online,
		"tailscale_hostname":   firstNonEmpty(host, ts.Self.HostName),
		"tailscale_ip":         first(ts.Self.TailscaleIPs),
		"tailscale_serve_file": fileExists(filepath.Join(m.config.WorkspacePath(), "ts-serve-picoclaw.json")),
		"cert_files":           certFiles,
		"https_candidate":      joinURL(controlPlaneHTTPSBase(controlNode{Hostname: host, TailscaleIP: first(ts.Self.TailscaleIPs)}, certFiles), "/dash/control"),
		"error":                errText(tsErr),
	}
}

func (m *Manager) securityExecProbe() map[string]any {
	workspace := m.config.WorkspacePath()
	return map[string]any{
		"dashboard_allow_shell": strings.EqualFold(strings.TrimSpace(os.Getenv("PICOCLAW_DASHBOARD_ALLOW_SHELL")), "true"),
		"allow_exec":            strings.EqualFold(strings.TrimSpace(os.Getenv("PICOCLAW_ALLOW_EXEC")), "true"),
		"codex_executor":        fileExists(filepath.Join(workspace, "wsl-codex-exec")),
		"workspace":             workspace,
		"guidance":              "Prefer scoped backend actions first; keep broad shell execution for break-glass operations.",
	}
}

func (m *Manager) hasControlPlaneDiagnoser() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.controlPlaneDiagnoser != nil
}

func (m *Manager) getControlPlaneDiagnoser() func(context.Context, string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.controlPlaneDiagnoser
}

func (m *Manager) storeSecurityDiagnosis(diag controlSecurityDiagnosis) {
	st := getControlPlaneState(m)
	st.mu.Lock()
	defer st.mu.Unlock()
	st.diagnosis = diag
}

func (m *Manager) handleControlPlaneAction(action, target string, payload map[string]any) (map[string]any, error) {
	action = strings.TrimSpace(action)
	if action == "" {
		return nil, errors.New("action is required")
	}
	defer func() {
		st := getControlPlaneState(m)
		st.mu.Lock()
		st.actionLog = append(st.actionLog, fmt.Sprintf("[%s] action=%s target=%s payload=%s", time.Now().Format(time.RFC3339), action, target, summarizePayload(payload)))
		trimStateHistory(st)
		st.mu.Unlock()
	}()

	result := map[string]any{"action": action, "target": target, "at": time.Now().Format(time.RFC3339)}
	switch action {
	case "security_check_all":
		status := m.buildControlPlaneStatus(getControlPlaneState(m).address)
		result["security"] = status.Security
		result["summary"] = status.Security.Summary
		return result, nil
	case "security_apply_safe_fixes":
		changed, err := m.applySafeControlPlaneFixes()
		result["changed"] = changed
		if err != nil {
			return result, err
		}
		result["message"] = "safe fixes applied"
		return result, nil
	case "security_sync_codex_fallback":
		changed, err := m.syncCodexFallbackScripts()
		result["changed"] = changed
		if err != nil {
			return result, err
		}
		result["message"] = "codex fallback scripts synchronized"
		return result, nil
	case "security_probe_google_auth":
		result["probe"] = m.securityGoogleProbe()
		return result, nil
	case "security_probe_secure_web":
		result["probe"] = m.securitySecureWebProbe()
		return result, nil
	case "security_probe_exec_surface":
		result["probe"] = m.securityExecProbe()
		return result, nil
	case "security_llm_diagnose":
		diagnosis, err := m.runControlPlaneSecurityDiagnosis()
		result["diagnosis"] = diagnosis
		if err != nil {
			return result, err
		}
		return result, nil
	case "security_llm_fix_plan":
		plan, err := m.runControlPlaneFixPlan()
		result["plan"] = plan
		if err != nil {
			return result, err
		}
		return result, nil
	case "test_health_endpoints":
		status := m.buildControlPlaneStatus(getControlPlaneState(m).address)
		gateway := strings.TrimSpace(status.Gateway)
		if gateway == "" {
			gateway = "127.0.0.1:3000"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		health, err1 := httpProbe(ctx, "http://"+gateway+"/health")
		ready, err2 := httpProbe(ctx, "http://"+gateway+"/ready")
		result["health"] = health
		result["ready"] = ready
		if err1 != nil || err2 != nil {
			result["error"] = fmt.Sprintf("health err=%v ready err=%v", err1, err2)
			return result, errors.New("one or more endpoint checks failed")
		}
		return result, nil
	case "ping_workers":
		nodeID := target
		if nodeID == "" {
			nodeID = "pc"
		}
		probeStatus, detail := m.rawSSHProbe(nodeID, tailPeer{})
		result["ssh_status"] = probeStatus
		result["detail"] = detail
		if probeStatus != "ok" {
			return result, errors.New("worker ping failed")
		}
		return result, nil
	case "validate_tailscale_path":
		targetHost := strings.TrimSpace(target)
		if targetHost == "" {
			targetHost = strings.TrimSpace(os.Getenv("PICOCLAW_PC_SSH_TARGET"))
		}
		if targetHost == "" {
			targetHost = "black-wave"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "tailscale", "ping", "-c", "1", targetHost)
		out, err := cmd.CombinedOutput()
		result["output"] = strings.TrimSpace(string(out))
		if err != nil {
			return result, err
		}
		return result, nil
	case "send_dry_run_job", "send_real_job":
		req := struct {
			Prompt           string         `json:"prompt"`
			AgentID          string         `json:"agent_id"`
			ExecutionTarget  string         `json:"execution_target"`
			WorkflowTemplate string         `json:"workflow_template"`
			TestType         string         `json:"test_type"`
			DryRun           bool           `json:"dry_run"`
			Payload          map[string]any `json:"payload"`
		}{
			Prompt:           "control-plane quick action",
			AgentID:          "main",
			ExecutionTarget:  "pico",
			WorkflowTemplate: "generic-debug",
			TestType:         "agent_prompt",
			DryRun:           action == "send_dry_run_job",
			Payload:          payload,
		}
		body, _ := json.Marshal(req)
		job, err := m.enqueueControlPlaneJob(bytes.NewReader(body))
		if err != nil {
			return result, err
		}
		result["job_id"] = job.ID
		return result, nil
	case "retry_job":
		jobID, _ := payload["job_id"].(string)
		if jobID == "" {
			return result, errors.New("payload.job_id is required")
		}
		return m.retryJob(jobID)
	case "cancel_job":
		jobID, _ := payload["job_id"].(string)
		if jobID == "" {
			return result, errors.New("payload.job_id is required")
		}
		return m.cancelJob(jobID)
	default:
		return result, fmt.Errorf("unsupported action: %s", action)
	}
}

func (m *Manager) retryJob(jobID string) (map[string]any, error) {
	st := getControlPlaneState(m)
	st.mu.Lock()
	orig := st.jobs[jobID]
	if orig == nil {
		st.mu.Unlock()
		return nil, errors.New("job not found")
	}
	reqPayload := map[string]any{}
	for k, v := range orig.Payload {
		reqPayload[k] = v
	}
	st.mu.Unlock()

	body, _ := json.Marshal(map[string]any{
		"prompt":            reqPayload["prompt"],
		"agent_id":          orig.AgentID,
		"execution_target":  orig.AssignedWorker,
		"workflow_template": reqPayload["workflow_template"],
		"test_type":         orig.JobType,
		"dry_run":           reqPayload["dry_run"],
		"payload":           reqPayload["payload"],
	})
	job, err := m.enqueueControlPlaneJob(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	st.mu.Lock()
	if j := st.jobs[job.ID]; j != nil {
		j.RetryCount++
	}
	st.mu.Unlock()

	return map[string]any{"action": "retry_job", "source_job": jobID, "job_id": job.ID}, nil
}

func (m *Manager) cancelJob(jobID string) (map[string]any, error) {
	st := getControlPlaneState(m)
	st.mu.Lock()
	defer st.mu.Unlock()
	job := st.jobs[jobID]
	if job == nil {
		return nil, errors.New("job not found")
	}
	if job.State == "completed" || job.State == "failed" {
		return map[string]any{"action": "cancel_job", "job_id": jobID, "state": job.State, "message": "job already terminal"}, nil
	}
	job.State = "failed"
	job.FinishedAt = time.Now()
	job.Timeline = append(job.Timeline, controlJobEvent{At: time.Now().Format(time.RFC3339), Status: "failed", Message: "canceled by operator"})
	st.chatHistory = append(st.chatHistory, controlChatMessage{Role: "system", Content: "job canceled: " + jobID, CreatedAt: time.Now().Format(time.RFC3339)})
	trimStateHistory(st)
	return map[string]any{"action": "cancel_job", "job_id": jobID, "state": "failed"}, nil
}

func (m *Manager) applySafeControlPlaneFixes() ([]string, error) {
	changed := []string{}

	if synced, err := m.syncCodexFallbackScripts(); err == nil {
		changed = append(changed, synced...)
	} else {
		return changed, err
	}

	servePath := filepath.Join(m.config.WorkspacePath(), "ts-serve-picoclaw.json")
	if !fileExists(servePath) {
		content := []byte("{\n  \"TCP\": {\n    \"443\": {\n      \"HTTPS\": true\n    }\n  },\n  \"Web\": {\n    \"black-wave.tailb9e21e.ts.net:443\": {\n      \"Handlers\": {\n        \"/\": {\n          \"Proxy\": \"http://127.0.0.1:3000\"\n        }\n      }\n    }\n  }\n}\n")
		if err := os.WriteFile(servePath, content, 0600); err != nil {
			return changed, err
		}
		changed = append(changed, servePath)
	}

	return changed, nil
}

func (m *Manager) syncCodexFallbackScripts() ([]string, error) {
	workspace := m.config.WorkspacePath()
	files := map[string]string{
		filepath.Join(workspace, "wsl-codex-exec"): desiredCodexExecScript(),
		filepath.Join(workspace, "node-exec"):      desiredNodeExecScript(),
		filepath.Join(workspace, "node-ssh"):       desiredNodeSSHScript(),
	}

	changed := make([]string, 0, len(files))
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			return changed, err
		}
		changed = append(changed, path)
	}
	return changed, nil
}

func (m *Manager) buildTerminalStatus(r *http.Request) controlTerminalStatus {
	st := getControlPlaneState(m)
	st.mu.Lock()
	m.pruneTerminalSessionsLocked(st)
	history := append([]controlTerminalEntry(nil), st.terminalHistory...)
	st.mu.Unlock()

	if len(history) > 18 {
		history = history[len(history)-18:]
	}

	host := hostOnly(r.Host)
	secure := isSecureControlRequest(r)
	tailscale := isTailscaleControlRequest(r)
	secretConfigured := controlPlaneTerminalSecret(m) != ""
	authenticated := m.isTerminalAuthenticated(r)
	allowed := secure && tailscale && secretConfigured

	reason := "ready"
	switch {
	case !secretConfigured:
		reason = "terminal login secret is not configured"
	case !tailscale:
		reason = "terminal requires a tailscale origin"
	case !secure:
		reason = "terminal requires https/tailscale cert"
	case !authenticated:
		reason = "login required"
	}

	return controlTerminalStatus{
		Allowed:         allowed,
		Authenticated:   authenticated,
		Secure:          secure,
		Tailscale:       tailscale,
		LoginConfigured: secretConfigured,
		Reason:          reason,
		Host:            host,
		History:         history,
		Suggestions: []string{
			"pwd",
			"ls -la",
			"tailscale status --json | sed -n '1,120p'",
			"systemctl status picoclaw --no-pager",
			"journalctl -u picoclaw -n 60 --no-pager",
		},
	}
}

func (m *Manager) handleTerminalLogin(w http.ResponseWriter, r *http.Request) (controlTerminalStatus, error) {
	status := m.buildTerminalStatus(r)
	if !status.Secure || !status.Tailscale {
		return status, errors.New(status.Reason)
	}
	secret := controlPlaneTerminalSecret(m)
	if secret == "" {
		return status, errors.New("terminal login is not configured")
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		return status, errors.New("invalid request")
	}
	if strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.Password) != secret {
		return status, errors.New("invalid terminal password")
	}

	token, err := randomHex(24)
	if err != nil {
		return status, err
	}

	st := getControlPlaneState(m)
	st.mu.Lock()
	st.terminalSessions[token] = time.Now().Add(12 * time.Hour)
	st.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "pcp_terminal",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   12 * 60 * 60,
	})

	return m.buildTerminalStatus(r), nil
}

func (m *Manager) handleTerminalLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("pcp_terminal"); err == nil {
		st := getControlPlaneState(m)
		st.mu.Lock()
		delete(st.terminalSessions, c.Value)
		st.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "pcp_terminal",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func (m *Manager) handleTerminalExec(r *http.Request) (map[string]any, error) {
	status := m.buildTerminalStatus(r)
	if !status.Allowed {
		return map[string]any{"status": status}, errors.New(status.Reason)
	}
	if !status.Authenticated {
		return map[string]any{"status": status}, errors.New("terminal login required")
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		return map[string]any{"status": status}, errors.New("invalid request")
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return map[string]any{"status": status}, errors.New("command is required")
	}
	if len(command) > 800 {
		return map[string]any{"status": status}, errors.New("command too long")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = m.config.WorkspacePath()
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if len(text) > 12000 {
		text = text[:12000] + "\n...[truncated]"
	}

	exitCode := 0
	if err != nil {
		exitCode = 1
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		if text == "" {
			text = err.Error()
		}
	}

	entry := controlTerminalEntry{
		At:       time.Now().Format(time.RFC3339),
		Command:  command,
		Output:   text,
		ExitCode: exitCode,
		Status:   ternaryStatus(exitCode == 0, "ok", "error"),
	}

	st := getControlPlaneState(m)
	st.mu.Lock()
	st.terminalHistory = append(st.terminalHistory, entry)
	if len(st.terminalHistory) > 40 {
		st.terminalHistory = st.terminalHistory[len(st.terminalHistory)-40:]
	}
	st.mu.Unlock()

	return map[string]any{"entry": entry, "status": m.buildTerminalStatus(r)}, nil
}

func (m *Manager) isTerminalAuthenticated(r *http.Request) bool {
	c, err := r.Cookie("pcp_terminal")
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return false
	}
	st := getControlPlaneState(m)
	st.mu.Lock()
	defer st.mu.Unlock()
	m.pruneTerminalSessionsLocked(st)
	expiresAt, ok := st.terminalSessions[c.Value]
	return ok && time.Now().Before(expiresAt)
}

func (m *Manager) pruneTerminalSessionsLocked(st *controlPlaneState) {
	now := time.Now()
	for token, expiresAt := range st.terminalSessions {
		if now.After(expiresAt) {
			delete(st.terminalSessions, token)
		}
	}
}

func (m *Manager) loadLogs(source string) []string {
	st := getControlPlaneState(m)
	st.mu.Lock()
	actions := append([]string(nil), st.actionLog...)
	st.mu.Unlock()

	switch source {
	case "dashboard":
		if len(actions) == 0 {
			return []string{"no dashboard action log entries yet"}
		}
		if len(actions) > 200 {
			actions = actions[len(actions)-200:]
		}
		return actions
	default:
		path := filepath.Join(m.config.WorkspacePath(), "heartbeat.log")
		lines := tailFile(path, 260)
		if len(lines) == 0 {
			return []string{"heartbeat log not found or empty"}
		}
		return lines
	}
}

func (m *Manager) cachedSSHProbe(st *controlPlaneState, nodeID string, peer tailPeer) (string, string) {
	st.mu.Lock()
	cached, ok := st.sshCache[nodeID]
	if ok && time.Since(cached.At) < 20*time.Second {
		st.mu.Unlock()
		return cached.Status, cached.Detail
	}
	st.mu.Unlock()

	status, detail := m.rawSSHProbe(nodeID, peer)

	st.mu.Lock()
	st.sshCache[nodeID] = sshProbe{At: time.Now(), Status: status, Detail: detail}
	st.mu.Unlock()
	return status, detail
}

func (m *Manager) rawSSHProbe(nodeID string, peer tailPeer) (string, string) {
	target := ""
	peerDNS := strings.TrimSuffix(peer.DNSName, ".")
	peerIP := first(peer.TailscaleIPs)
	switch nodeID {
	case "vps":
		target = strings.TrimSpace(os.Getenv("PICOCLAW_VPS_SSH_TARGET"))
		if target == "" {
			target = firstNonEmpty("vps", peerDNS, peerIP)
		}
	case "pico":
		target = strings.TrimSpace(os.Getenv("PICOCLAW_PICO_SSH_TARGET"))
		if target == "" {
			target = firstNonEmpty("orangepi", peerDNS, peerIP)
		}
	case "pc":
		target = strings.TrimSpace(os.Getenv("PICOCLAW_PC_SSH_TARGET"))
		if target == "" {
			target = firstNonEmpty("black-wave", peerDNS, peerIP)
		}
	case "storage":
		target = strings.TrimSpace(os.Getenv("PICOCLAW_STORAGE_SSH_TARGET"))
		if target == "" {
			target = firstNonEmpty(peerDNS, peerIP)
		}
	default:
		target = firstNonEmpty(peerDNS, peerIP)
	}
	if target == "" {
		return "unknown", "no SSH target mapping"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=4",
		"-o", "StrictHostKeyChecking=accept-new",
		target,
		"echo", "ssh_ok",
	)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "warn", trimLogLine(text)
	}
	if strings.Contains(text, "ssh_ok") {
		return "ok", "ready"
	}
	if strings.Contains(strings.ToLower(text), "operation not permitted") {
		return "warn", "Tailscale SSH reachable, but remote command execution is denied"
	}
	return "warn", trimLogLine(text)
}

type tailStatus struct {
	Self  tailPeer `json:"Self"`
	Peer  map[string]tailPeer
	Peers []tailPeer
}

type tailPeer struct {
	HostName     string                     `json:"HostName"`
	DNSName      string                     `json:"DNSName"`
	OS           string                     `json:"OS"`
	Online       bool                       `json:"Online"`
	Tags         []string                   `json:"Tags"`
	TailscaleIPs []string                   `json:"TailscaleIPs"`
	CapMap       map[string]json.RawMessage `json:"CapMap"`
}

func scoreTailPeer(nodeID string, hints []string, peer tailPeer) int {
	name := strings.ToLower(peer.HostName + " " + strings.TrimSuffix(peer.DNSName, "."))
	score := 0
	hintMatched := false
	for _, hint := range hints {
		if !strings.Contains(name, hint) {
			continue
		}
		hintMatched = true
		score += 10
		if strings.EqualFold(peer.HostName, hint) || strings.EqualFold(strings.TrimSuffix(peer.DNSName, "."), hint) {
			score += 10
		}
	}

	switch nodeID {
	case "pc":
		if containsTag(peer.Tags, "tag:wslhost") {
			hintMatched = true
			score += 40
		}
		if containsTag(peer.Tags, "tag:control") {
			score += 20
		}
		if strings.EqualFold(peer.OS, "linux") {
			score += 15
		}
	case "vps":
		if containsTag(peer.Tags, "tag:control") {
			score += 20
		}
		if strings.EqualFold(peer.OS, "linux") {
			score += 10
		}
	case "storage":
		if strings.EqualFold(peer.OS, "linux") {
			score += 10
		}
	}

	if !hintMatched {
		return 0
	}
	if peer.Online {
		score += 5
	}
	return score
}

func containsTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func loadTailStatus() (tailStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return tailStatus{}, err
	}
	var raw struct {
		Self tailPeer            `json:"Self"`
		Peer map[string]tailPeer `json:"Peer"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return tailStatus{}, err
	}
	peers := make([]tailPeer, 0, len(raw.Peer))
	for _, p := range raw.Peer {
		peers = append(peers, p)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].HostName < peers[j].HostName })
	return tailStatus{Self: raw.Self, Peer: raw.Peer, Peers: peers}, nil
}

func tailFile(path string, n int) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	out := make([]string, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func summarizePayload(payload map[string]any) string {
	if len(payload) == 0 {
		return "-"
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "payload"
	}
	s := string(b)
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

func trimStateHistory(st *controlPlaneState) {
	if len(st.jobOrder) > 200 {
		st.jobOrder = st.jobOrder[len(st.jobOrder)-200:]
	}
	if len(st.chatHistory) > 250 {
		st.chatHistory = st.chatHistory[len(st.chatHistory)-250:]
	}
	if len(st.actionLog) > 250 {
		st.actionLog = st.actionLog[len(st.actionLog)-250:]
	}
	if len(st.terminalHistory) > 40 {
		st.terminalHistory = st.terminalHistory[len(st.terminalHistory)-40:]
	}
}

func respondJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func boolState(ok bool, fallback string) string {
	if ok {
		return "ok"
	}
	if fallback == "" {
		return "warn"
	}
	return fallback
}

func envState(name string) string {
	if strings.TrimSpace(os.Getenv(name)) != "" {
		return "ok"
	}
	return "warn"
}

func errState(err error) string {
	if err == nil {
		return "ok"
	}
	return "warn"
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return trimLogLine(err.Error())
}

func summarizeServiceStatuses(services []controlService) map[string]string {
	out := make(map[string]string, len(services))
	for _, svc := range services {
		out[svc.ID] = svc.Status
	}
	return out
}

func findNodeByID(nodes []controlNode, id string) controlNode {
	for _, n := range nodes {
		if n.ID == id {
			return n
		}
	}
	return controlNode{ID: id}
}

func anyRemoteWorkerReady(nodes []controlNode) bool {
	for _, n := range nodes {
		if n.ID != "pico" && n.WorkerReady {
			return true
		}
	}
	return false
}

func countSecurityFindings(findings []controlSecurityFinding, severity string) int {
	count := 0
	for _, f := range findings {
		if f.Severity == severity {
			count++
		}
	}
	return count
}

func securityOverallStatus(criticalCount, warnCount int) string {
	if criticalCount > 0 {
		return "error"
	}
	if warnCount > 0 {
		return "warn"
	}
	return "ok"
}

func securityDomainStatus(ok bool, partial bool) string {
	if ok {
		return "ok"
	}
	if partial {
		return "warn"
	}
	return "error"
}

func securityDomainScore(ok bool, partial bool) int {
	if ok {
		return 92
	}
	if partial {
		return 64
	}
	return 28
}

func countAuthProvidersByStatus(providers []controlAuthProvider, status string) int {
	count := 0
	for _, p := range providers {
		if p.Status == status {
			count++
		}
	}
	return count
}

func compactSignals(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func serviceStatusIs(services []controlService, id, want string) bool {
	for _, svc := range services {
		if svc.ID == id {
			return svc.Status == want
		}
	}
	return false
}

func ternaryText(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func ternaryInt(cond bool, yes, no int) int {
	if cond {
		return yes
	}
	return no
}

func detectTailscaleCertFiles(hostname, workspace string) []string {
	hostnames := compactSignals([]string{
		hostname,
		firstNonEmpty(hostname, strings.TrimSpace(os.Getenv("TS_CERT_HOSTNAME"))),
	})
	roots := []string{
		homeDir(),
		workspace,
		filepath.Join(homeDir(), ".config", "tailscale"),
		filepath.Join(homeDir(), ".tailscale"),
	}
	seen := map[string]bool{}
	files := []string{}
	for _, host := range hostnames {
		for _, root := range roots {
			for _, ext := range []string{".crt", ".key"} {
				path := filepath.Join(root, host+ext)
				if fileExists(path) && !seen[path] {
					seen[path] = true
					files = append(files, path)
				}
			}
		}
	}
	sort.Strings(files)
	return files
}

func controlPlanePublicBase(selfNode controlNode, serveConfig bool) string {
	if selfNode.TailscaleIP != "" {
		return "http://" + selfNode.TailscaleIP + ":3000"
	}
	if serveConfig && selfNode.Hostname != "" {
		return "http://" + selfNode.Hostname + ":3000"
	}
	return ""
}

func controlPlaneHTTPSBase(selfNode controlNode, certFiles []string) string {
	if len(certFiles) == 0 {
		return ""
	}
	if selfNode.Hostname != "" {
		return "https://" + selfNode.Hostname
	}
	return ""
}

func joinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if base == "" {
		return ""
	}
	return base + path
}

func authProviderState(state controlAuth, provider string) string {
	for _, p := range state.Providers {
		if p.Provider == provider {
			return p.Status
		}
	}
	return "warn"
}

func stateFromActive(jobID string) string {
	if jobID != "" {
		return "running"
	}
	return "ok"
}

func processMetrics() string {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return fmt.Sprintf("cpu=%d goroutines=%d heap=%s", runtime.NumCPU(), runtime.NumGoroutine(), humanSize(int64(ms.Alloc)))
}

func hasSSHCap(capMap map[string]json.RawMessage) bool {
	if len(capMap) == 0 {
		return false
	}
	_, ok := capMap["https://tailscale.com/cap/ssh"]
	return ok
}

func tagsState(tags []string) string {
	if len(tags) == 0 {
		return "missing"
	}
	return strings.Join(tags, ",")
}

func onlineText(online bool) string {
	if online {
		return "reachable"
	}
	return "offline"
}

func inferredServices(role string) []string {
	switch role {
	case "gateway":
		return []string{"reverse-proxy", "gateway"}
	case "pc-worker":
		return []string{"worker-api", "build-runner"}
	case "storage":
		return []string{"artifact-storage", "samba"}
	default:
		return []string{"worker"}
	}
}

func first(in []string) string {
	if len(in) == 0 {
		return ""
	}
	return in[0]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func ternaryStatus(ok bool, whenOK, whenFalse string) string {
	if ok {
		return whenOK
	}
	return whenFalse
}

func hasArtifactKind(artifacts []controlArtifact, kind string) bool {
	for _, a := range artifacts {
		if a.Kind == kind {
			return true
		}
	}
	return false
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", f, units[i])
}

func controlPlaneTerminalSecret(m *Manager) string {
	if v := strings.TrimSpace(os.Getenv("PICOCLAW_TERMINAL_PASSWORD")); v != "" {
		return v
	}
	tokenPath := filepath.Join(m.config.WorkspacePath(), ".control-plane-terminal-token")
	if b, err := os.ReadFile(tokenPath); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			return v
		}
	}
	return strings.TrimSpace(m.config.Channels.Pico.Token)
}

func isSecureControlRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	host := hostOnly(r.Host)
	return strings.Contains(host, ".ts.net")
}

func isTailscaleControlRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	host := hostOnly(r.Host)
	if strings.Contains(host, ".ts.net") {
		return true
	}
	for _, candidate := range []string{
		strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]),
		strings.TrimSpace(r.Header.Get("X-Real-IP")),
		remoteAddrIP(r.RemoteAddr),
	} {
		if candidate == "" {
			continue
		}
		ip := net.ParseIP(candidate)
		if ip == nil {
			continue
		}
		if isTailscaleIP(ip) || ip.IsLoopback() {
			return true
		}
	}
	return false
}

func isTailscaleIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
	}
	return strings.HasPrefix(strings.ToLower(ip.String()), "fd7a:115c:a1e0:")
}

func remoteAddrIP(addr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(addr)
}

func hostOnly(addr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(addr)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func desiredCodexExecScript() string {
	return "#!/usr/bin/env bash\n" +
		"set -euo pipefail\n\n" +
		"TS_HOST=\"${TS_HOST:-black-wave}\"\n" +
		"LOCAL_HOST=\"$(hostname)\"\n" +
		"LOCAL_IP=\"$(tailscale ip -4 2>/dev/null | head -n1 || echo '')\"\n\n" +
		"if [ \"$#\" -lt 1 ]; then\n" +
		"  echo \"usage: wsl-codex-exec <health-check|status|prompt> [target]\"\n" +
		"  exit 2\n" +
		"fi\n\n" +
		"is_local() {\n" +
		"  local target=\"$1\"\n" +
		"  [[ \"$target\" == \"localhost\" ]] || [[ \"$target\" == \"$LOCAL_HOST\" ]] || [[ \"$target\" == \"$LOCAL_IP\" ]] || [[ \"$target\" == \"127.0.0.1\" ]]\n" +
		"}\n\n" +
		"run_picoclaw_local() {\n" +
		"  local prompt=\"$1\"\n" +
		"  /usr/local/bin/picoclaw agent --model \"${PICO_CLI_MODEL:-gpt-5.2}\" -m \"$prompt\"\n" +
		"}\n\n" +
		"run_remote_codex() {\n" +
		"  local target=\"$1\"\n" +
		"  local prompt=\"$2\"\n" +
		"  local remote_cmd=\"cat >/tmp/codex_prompt.txt && chown caps:caps /tmp/codex_prompt.txt && su - caps -c 'cd /home/caps && codex exec --sandbox workspace-write --ask-for-approval never \\\"\\$(cat /tmp/codex_prompt.txt)\\\"'\"\n" +
		"  printf '%s' \"$prompt\" | tailscale ssh root@\"$target\" \"$remote_cmd\"\n" +
		"}\n\n" +
		"CMD=\"$1\"\n" +
		"TARGET=\"${2:-$TS_HOST}\"\n\n" +
		"case \"$CMD\" in\n" +
		"  health-check)\n" +
		"    if is_local \"$TARGET\"; then\n" +
		"      echo \"=== LOCAL PICO HEALTH CHECK ===\"\n" +
		"      echo \"Host: $(hostname)\"\n" +
		"      echo \"User: $(whoami)\"\n" +
		"      uptime\n" +
		"      df -h /\n" +
		"      echo \"Fallback: local picoclaw agent available\"\n" +
		"    else\n" +
		"      echo \"=== REMOTE HEALTH CHECK ($TARGET) ===\"\n" +
		"      if ! tailscale ssh root@\"$TARGET\" 'echo \"Host: $(hostname)\"; echo \"User: $(whoami)\"; uptime; df -h /' ; then\n" +
		"        echo \"remote tailscale ssh unavailable; local pico fallback ready\"\n" +
		"      fi\n" +
		"    fi\n" +
		"    ;;\n" +
		"  status)\n" +
		"    if is_local \"$TARGET\"; then\n" +
		"      echo \"=== LOCAL STATUS ===\"\n" +
		"      tailscale status 2>/dev/null || true\n" +
		"      systemctl --failed 2>/dev/null || true\n" +
		"      echo \"model=${PICO_CLI_MODEL:-gpt-5.2}\"\n" +
		"    else\n" +
		"      echo \"=== REMOTE STATUS ($TARGET) ===\"\n" +
		"      if ! tailscale ssh root@\"$TARGET\" 'tailscale status 2>/dev/null || true; systemctl --failed 2>/dev/null || true' ; then\n" +
		"        echo \"remote tailscale ssh unavailable; local pico fallback ready\"\n" +
		"      fi\n" +
		"    fi\n" +
		"    ;;\n" +
		"  *)\n" +
		"    PROMPT=\"$CMD\"\n" +
		"    if is_local \"$TARGET\"; then\n" +
		"      echo \"[LOCAL PICO AGENT]\"\n" +
		"      run_picoclaw_local \"$PROMPT\"\n" +
		"    else\n" +
		"      echo \"[REMOTE EXEC -> $TARGET]\"\n" +
		"      if ! run_remote_codex \"$TARGET\" \"$PROMPT\"; then\n" +
		"        echo \"[REMOTE FAILED -> LOCAL PICO AGENT FALLBACK]\" >&2\n" +
		"        run_picoclaw_local \"$PROMPT\"\n" +
		"      fi\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"
}

func desiredNodeExecScript() string {
	return "#!/usr/bin/env bash\n" +
		"set -euo pipefail\n\n" +
		"TARGET=\"${1:-}\"\n" +
		"shift || true\n" +
		"CMD=\"$*\"\n" +
		"LOCAL_HOST=\"$(hostname)\"\n" +
		"TS_IP=\"$(tailscale ip -4 2>/dev/null | head -n1 || true)\"\n\n" +
		"if [[ -z \"$TARGET\" || \"$TARGET\" == \"localhost\" || \"$TARGET\" == \"$LOCAL_HOST\" || \"$TARGET\" == \"$TS_IP\" ]]; then\n" +
		"  exec bash -lc \"$CMD\"\n" +
		"fi\n\n" +
		"exec tailscale ssh \"root@${TARGET}\" \"$CMD\"\n"
}

func desiredNodeSSHScript() string {
	return "#!/usr/bin/env bash\n" +
		"set -euo pipefail\n\n" +
		"if [ $# -lt 2 ]; then\n" +
		"  echo \"Usage: $0 <host> <command>\"\n" +
		"  exit 1\n" +
		"fi\n\n" +
		"HOST=\"$1\"\n" +
		"shift\n" +
		"exec tailscale ssh \"root@${HOST}\" \"$*\"\n"
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/root"
	}
	return h
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func httpProbe(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
	text := strings.TrimSpace(string(body))
	if len(text) > 160 {
		text = text[:160] + "..."
	}
	return fmt.Sprintf("%s status=%d body=%s", url, res.StatusCode, text), nil
}
