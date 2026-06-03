<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { openModal } from '../lib/modals.svelte';
  import Icon from './ui/Icon.svelte';
  import WtGroup from './WtGroup.svelte';
</script>

<div class="detail-body">
  <div class="wt-groups">
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
