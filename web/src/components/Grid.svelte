<script lang="ts">
  import type { Session } from '../lib/types';
  import { app, MAX_GRID } from '../lib/state.svelte';
  import { api } from '../lib/api';
  import { agentMeta, statusMeta } from '../lib/util';
  import { dndzone } from 'svelte-dnd-action';
  import BranchLabel from './ui/BranchLabel.svelte';
  import TermPane from './TermPane.svelte';

  // default grid: alive sessions by grid_order, capped at MAX_GRID
  function defaultGrid(): string[] {
    return app.sessions
      .filter((s) => s.alive)
      .slice()
      .sort((a, b) => (a.grid_order || 0) - (b.grid_order || 0))
      .slice(0, MAX_GRID)
      .map((s) => s.id);
  }

  // on mount, seed the grid when empty
  $effect(() => {
    if (app.cur.grid.length === 0) {
      const g = defaultGrid();
      if (g.length) app.cur.grid = g;
    }
  });

  const full = $derived(app.cur.grid.length >= MAX_GRID);
  // sessions currently shown in the grid, in grid order
  const panes = $derived(
    app.cur.grid
      .map((id) => app.sessions.find((s) => s.id === id))
      .filter((s): s is Session => Boolean(s))
  );

  const sessName = (s: Session) => ((s.name || '').trim()) || agentMeta(s.agent).label;
  const wtName = (path: string) => app.worktrees.find((w) => w.path === path)?.name ?? '';

  function toggle(id: string) {
    if (app.cur.grid.includes(id)) {
      app.cur.grid = app.cur.grid.filter((g) => g !== id);
    } else if (app.cur.grid.length < MAX_GRID) {
      app.cur.grid = [...app.cur.grid, id];
    }
  }

  // dnd over panes (each pane is a Session with a stable id); reorder app.cur.grid
  function onConsider(e: CustomEvent<{ items: Session[] }>) {
    app.cur.grid = e.detail.items.map((s) => s.id);
  }
  function onFinalize(e: CustomEvent<{ items: Session[] }>) {
    app.cur.grid = e.detail.items.map((s) => s.id);
    api.orderGrid(app.cur.grid);
  }
</script>

<div class="detail-body">
  <div class="grid-sel">
    <span class="lbl">Show in grid:</span>
    <div class="gchips">
      {#if app.sessions.length}
        {#each app.sessions as s (s.id)}
          {@const on = app.cur.grid.includes(s.id)}
          {@const disabled = !on && full}
          <button
            class="gchip"
            class:on
            class:disabled
            onclick={() => { if (!disabled) toggle(s.id); }}
          >
            <span class="dot {statusMeta(s.status).cls}"></span>{sessName(s)}<span class="br" title={wtName(s.worktree)}><BranchLabel name={wtName(s.worktree)} /></span>
          </button>
        {/each}
      {:else}
        <span class="faint">no sessions</span>
      {/if}
    </div>
    <span class="cap">{app.cur.grid.length}/{MAX_GRID}</span>
  </div>

  {#if app.cur.grid.length === 0}
    <div class="empty">
      <div class="big">Select sessions for the grid</div>
      <div>Up to {MAX_GRID} terminals at once.</div>
    </div>
  {:else}
    <div class="term-grid" class:cols-1={app.cur.grid.length === 1}
         use:dndzone={{ items: panes, flipDurationMs: 140 }} onconsider={onConsider} onfinalize={onFinalize}>
      {#each panes as s (s.id)}
        <TermPane session={s} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .detail-body { padding: 20px 24px 48px; }

  .grid-sel { display: flex; align-items: flex-start; gap: 10px; margin-bottom: 14px; padding: 12px 15px; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); }
  .grid-sel .lbl { font-size: 11px; color: var(--text-dim); padding-top: 6px; white-space: nowrap; }
  .gchips { flex: 1; min-width: 0; display: flex; flex-wrap: wrap; gap: 8px; max-height: 80px; overflow-y: auto; padding-right: 2px; }
  .grid-sel .cap { font-size: 11px; color: var(--text-faint); font-family: var(--mono); padding-top: 6px; white-space: nowrap; }
  .gchip { display: flex; align-items: center; gap: 7px; white-space: nowrap; padding: 5px 10px; border-radius: var(--radius-sm); border: 1px solid var(--border-2); background: var(--surface-2); font-size: 11.5px; color: var(--text-dim); cursor: pointer; transition: background .1s, border-color .1s, color .1s, opacity .1s; }
  .gchip:hover:not(.disabled) { color: var(--text); border-color: var(--text-dim); }
  .gchip.on { background: var(--accent-bg); border-color: var(--accent-dim); color: var(--text); }
  .gchip.disabled { opacity: .35; cursor: not-allowed; }
  /* grid selector chip: chip + truncated rest */
  .gchip .br { display: inline-flex; align-items: center; gap: 5px; min-width: 0; }
  .gchip :global(.b-tag) { font-size: 8.5px; padding: 0 3px; }
  .gchip :global(.b-rest) { font-size: 10px; color: var(--text-faint); max-width: 130px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .gchip.on :global(.b-rest) { color: var(--text-dim); }

  .faint { color: var(--text-faint); }

  /* status dot */
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.idle { background: var(--idle); }
  .dot.wait { background: var(--wait); }
  .dot.stopped { background: #3a3a42; }

  /* empty state */
  .empty { text-align: center; padding: 70px 20px; color: var(--text-faint); }
  .empty .big { font-size: 15px; color: var(--text-dim); margin-bottom: 6px; }

  /* grid of terminals */
  .term-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
  .term-grid.cols-1 { grid-template-columns: 1fr; }
</style>
