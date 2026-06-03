<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { api } from '../lib/api';
  import { openModal } from '../lib/modals.svelte';
  import { dndzone } from 'svelte-dnd-action';
  import type { Worktree } from '../lib/types';
  import Icon from './ui/Icon.svelte';
  import WtGroup from './WtGroup.svelte';

  // svelte-dnd-action needs a stable `id`; worktrees key on `path`.
  type WtItem = Worktree & { id: string };
  const items = $derived(app.worktrees.map((wt): WtItem => ({ ...wt, id: wt.path })));

  function onConsider(e: CustomEvent<{ items: WtItem[] }>) {
    app.worktrees = e.detail.items;
  }
  function onFinalize(e: CustomEvent<{ items: WtItem[] }>) {
    app.worktrees = e.detail.items;
    if (app.cur.projectId) api.orderWorktrees(app.cur.projectId, app.worktrees.map((w) => w.path));
  }
</script>

<div class="detail-body">
  <div class="wt-groups" use:dndzone={{ items, flipDurationMs: 140, type: 'wt' }} onconsider={onConsider} onfinalize={onFinalize}>
    {#each items as wt (wt.path)}
      <WtGroup {wt} />
    {/each}
  </div>
  <button class="btn ghost sm" style="margin-top:6px" onclick={() => openModal('newWorktree')}><Icon name="plus" size={13} /> New worktree</button>
</div>

<style>
  .detail-body { padding: 20px 24px 48px; }
  .wt-groups { display: block; }
</style>
