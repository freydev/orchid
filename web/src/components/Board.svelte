<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { api } from '../lib/api';
  import { openModal } from '../lib/modals.svelte';
  import { sortable } from '../actions/sortable';
  import { reorderBy } from '../lib/util';
  import Icon from './ui/Icon.svelte';
  import WtGroup from './WtGroup.svelte';

  function onOrder(ids: string[]) {
    app.worktrees = reorderBy(app.worktrees, ids, (w) => w.path);
    if (app.cur.projectId) api.orderWorktrees(app.cur.projectId, ids);
  }
</script>

<div class="detail-body">
  <div class="wt-groups" use:sortable={{ itemSel: '.wt-group', handle: '.grip-wt', getId: (el) => el.dataset.wtpath!, onOrder }}>
    {#each app.worktrees as wt (wt.path)}
      <WtGroup {wt} />
    {/each}
  </div>
  <button class="btn ghost sm" style="margin-top:6px" onclick={() => openModal('newWorktree')}><Icon name="plus" size={13} /> New worktree</button>
</div>

<style>
  .detail-body { padding: 20px 24px 48px; }
  .wt-groups { display: block; }
</style>
