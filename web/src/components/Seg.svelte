<script lang="ts">
  import type { View } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import Icon from './ui/Icon.svelte';

  const items: { v: View; ic: string; label: string }[] = [
    { v: 'list', ic: 'list', label: 'Board' },
    { v: 'grid', ic: 'grid', label: 'Grid' },
    { v: 'terminal', ic: 'term', label: 'Terminal' },
  ];

  function pick(v: View) {
    if (app.cur.view === v) return;
    app.cur.view = v;
    setHash();
  }
</script>

<div class="seg">
  {#each items as it}
    <button class={app.cur.view === it.v ? 'active' : ''} data-view={it.v} onclick={() => pick(it.v)}>
      <Icon name={it.ic} size={14} /> {it.label}
    </button>
  {/each}
</div>

<style>
  .seg { display: flex; background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 2px; gap: 2px; }
  .seg button { padding: 4px 10px; border-radius: var(--radius-sm); font-size: 12px; color: var(--text-dim); display: flex; align-items: center; gap: 6px; letter-spacing: -0.01em; }
  .seg button.active { background: var(--accent-bg); color: var(--accent); font-weight: 600; }
  .seg button:hover:not(.active) { color: var(--text); }
</style>
