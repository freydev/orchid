<script lang="ts">
  // bottom status bar (tmux/vim style): mode · context · session counts — ported from web/app.js
  import { app } from '../lib/state.svelte';
  import { sendToTerminal } from '../lib/terminalInput';
  import type { Session } from '../lib/types';

  // arrow-key escape sequences (mobile keyboards lack arrow keys)
  const ARROW_UP = '\x1b[A';
  const ARROW_DOWN = '\x1b[B';
  const ENTER = '\r';
  const onTermTab = $derived(!!app.cur.projectId);

  const sb = $derived.by(() => {
    let list: Session[];
    let ctxProject: string | null = null;
    let ctxText: string;
    if (app.cur.projectId) {
      list = app.sessions;
      const proj = app.projects.find((p) => p.id === app.cur.projectId);
      ctxProject = proj ? proj.name : "project";
      ctxText = list.length + " session" + (list.length === 1 ? "" : "s");
    } else {
      list = app.projects.flatMap((p) => p.session_list || []);
      ctxProject = null;
      ctxText = app.projects.length + " project" + (app.projects.length === 1 ? "" : "s");
    }
    return {
      ctxProject,
      ctxText,
      running: list.filter((s) => s.status === "running").length,
      waiting: list.filter((s) => s.status === "requires_action").length,
      idle: list.filter((s) => s.status === "idle" || s.status === "stopped").length,
    };
  });
</script>

<div class="statusbar">
  <div class="sb-seg sb-mode">Orchid</div>
  <div class="sb-seg sb-ctx">
    {#if sb.ctxProject !== null}<span class="accent">{sb.ctxProject}</span> · {sb.ctxText}{:else}{sb.ctxText}{/if}
  </div>
  <div class="spacer"></div>
  {#if onTermTab}
    <div class="sb-keys">
      <button class="sb-key" title="Arrow up" aria-label="Arrow up" onclick={() => sendToTerminal(app.cur.activeId, ARROW_UP)}>↑</button>
      <button class="sb-key" title="Arrow down" aria-label="Arrow down" onclick={() => sendToTerminal(app.cur.activeId, ARROW_DOWN)}>↓</button>
      <button class="sb-key wide" title="Enter" aria-label="Enter" onclick={() => sendToTerminal(app.cur.activeId, ENTER)}>⏎</button>
    </div>
  {/if}
  <div class="sb-counts">
    <span class="sb-count ok"><span class="dot ok"></span>{sb.running} run</span>
    <span class="sb-count wait"><span class="dot wait"></span>{sb.waiting} wait</span>
    <span class="sb-count idle"><span class="dot idle"></span>{sb.idle} idle</span>
  </div>
</div>

<style>
  .statusbar { display: flex; align-items: stretch; height: var(--statusbar-h); flex-shrink: 0; border-top: 1px solid var(--border); background: var(--surface); font-size: 11px; letter-spacing: -0.01em; position: relative; z-index: 1; }
  .sb-seg { display: flex; align-items: center; gap: 7px; padding: 0 12px; color: var(--text-dim); white-space: nowrap; }
  .sb-mode { background: var(--accent); color: var(--bg); font-weight: 700; letter-spacing: -0.01em; }
  .sb-ctx { color: var(--text); border-right: 1px solid var(--border); min-width: 0; overflow: hidden; text-overflow: ellipsis; }
  .sb-ctx .accent { color: var(--accent); }
  .statusbar .spacer { flex: 1; }
  .sb-counts { display: flex; align-items: center; gap: 12px; padding: 0 12px; border-left: 1px solid var(--border); }
  .sb-count { display: inline-flex; align-items: center; gap: 6px; }
  .sb-count.ok { color: var(--ok); } .sb-count.wait { color: var(--wait); } .sb-count.idle { color: var(--text-dim); }
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.wait { background: var(--wait); }
  .dot.idle { background: var(--idle); }

  /* mobile terminal arrow keys — hidden on desktop (hardware keyboard has them) */
  .sb-keys { display: none; }
  .sb-key { display: inline-flex; align-items: center; justify-content: center; height: 100%; min-width: 46px; padding: 0 12px; font-size: 16px; line-height: 1; color: var(--text); border-left: 1px solid var(--border); }
  .sb-key:active { background: var(--accent-bg); color: var(--accent); }
  .sb-key.wide { min-width: 54px; font-size: 18px; }

  @media (max-width: 640px) {
    .sb-ctx { display: none; }
    .sb-seg { padding: 0 10px; }
    .sb-keys { display: flex; }
    .sb-counts { gap: 10px; padding: 0 10px; }
  }
</style>
