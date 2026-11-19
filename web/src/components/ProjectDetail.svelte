<script lang="ts">
  import type { Project } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import DetailBar from './DetailBar.svelte';
  import Terminal from './Terminal.svelte';

  const project = $derived<Project>(
    app.projects.find((p) => p.id === app.cur.projectId) ??
      ({ name: '', path: '' } as Project)
  );
</script>

<div class="wrap focus-mode">
  <DetailBar {project} />
  <Terminal />
</div>

<style>
  .wrap { max-width: none; margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100%; }
  .wrap.focus-mode { height: calc(100vh - 52px - var(--statusbar-h)); overflow: hidden; }
  .wrap.focus-mode :global(.focus-term) { border-radius: var(--radius-sm); border: none; background: var(--term-bg); }
  .wrap.focus-mode :global(.focus-rail) { border-radius: var(--radius-sm); border: none; border-right: 1px solid var(--border); background: var(--surface); padding: 8px 8px 8px 10px; }
  .wrap.focus-mode :global(.focus-view) { padding: 0; gap: 0; flex: 1; min-height: 0; height: auto; }
  .wrap.focus-mode :global(.term-body) { padding: 14px 18px; }
</style>
