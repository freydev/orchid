<script lang="ts">
  import type { Project } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import DetailBar from './DetailBar.svelte';
  import Board from './Board.svelte';
  import Terminal from './Terminal.svelte';

  const project = $derived<Project>(
    app.projects.find((p) => p.id === app.cur.projectId) ??
      ({ name: '', path: '' } as Project)
  );
  const focus = $derived(app.cur.view === 'terminal');
</script>

<div class="wrap" class:focus-mode={focus}>
  <DetailBar {project} />
  {#if app.cur.view === 'list'}
    <Board />
  {:else if app.cur.view === 'grid'}
    <div class="detail-body">grid</div>
  {:else}
    <Terminal />
  {/if}
</div>

<style>
  .wrap { max-width: none; margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100%; }
  .detail-body { padding: 20px 24px 48px; }
  .wrap.focus-mode { height: calc(100vh - 52px - var(--statusbar-h)); overflow: hidden; }
  .wrap.focus-mode :global(.focus-term) { border-radius: var(--radius-sm); border: none; background: var(--term-bg); }
  .wrap.focus-mode :global(.focus-rail) { border-radius: var(--radius-sm); border: none; border-right: 1px solid var(--border); background: var(--surface); padding: 8px 8px 8px 10px; }
  .wrap.focus-mode :global(.focus-view) { padding: 0; gap: 0; flex: 1; min-height: 0; height: auto; }
  .wrap.focus-mode :global(.term-body) { padding: 14px 18px; }
</style>
