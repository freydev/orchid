<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { app } from '../../lib/state.svelte';
  import Icon from '../ui/Icon.svelte';
  let { data }: { data: any } = $props();
  const p = $derived(data.project);
  const wt = $derived(data.wt);
  const expanded = $derived(data.cb.expandedWorktrees().has(wt.path));
  const removing = $derived(app.removingWt.has(wt.path));
  const folder = $derived((wt.path || '').split('/').filter(Boolean).pop() || wt.name || '');
  const branch = $derived((wt.branch || '').trim());
</script>

<div class="gnode worktree" class:removing>
  <Handle type="target" position={Position.Left} />
  <div class="row">
    <button class="caret" class:open={expanded} title="Expand" onclick={() => data.cb.toggleWorktree(wt.path)}><Icon name="chevR" size={12} /></button>
    <span class="folder" title={wt.path}><Icon name="folder" size={13} />{folder}</span>
    <div class="spacer"></div>
    <button class="ic" title="View diff" onclick={() => data.cb.openDiff(p, wt)}><Icon name="diff" size={14} /></button>
    <button class="ic" title="New session" onclick={() => data.cb.openNewSession(p, wt)}><Icon name="plus" size={13} /></button>
    {#if !wt.is_main && !wt.primary}
      <button class="ic danger" title="Remove worktree" onclick={() => data.cb.removeWorktree(p, wt)}><Icon name="x" size={14} /></button>
    {/if}
  </div>
  {#if branch}
    <div class="brow"><span class="wbranch" title="git branch"><Icon name="branch" size={11} />{branch}</span></div>
  {/if}
  <Handle type="source" position={Position.Right} />
</div>

<style>
  /* WORKTREE = light "branch" connector: flat, dashed outline, cyan accent */
  .gnode { background: color-mix(in srgb, var(--surface) 55%, transparent); border: 1px dashed var(--border-2); border-left: 3px solid #57b6c9; border-radius: var(--radius-sm); padding: 10px 12px; box-sizing: border-box; width: 300px; height: 84px; display: flex; flex-direction: column; gap: 8px; }
  .gnode.removing { opacity: .55; pointer-events: none; }
  .row { display: flex; align-items: center; gap: 6px; width: 100%; }
  .caret { color: var(--text-faint); transition: transform .15s; display: inline-flex; flex-shrink: 0; } .caret.open { transform: rotate(90deg); }
  .folder { display: inline-flex; align-items: center; gap: 6px; min-width: 0; font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .folder > :global(svg) { color: #57b6c9; flex-shrink: 0; }
  .spacer { flex: 1; }
  .brow { display: flex; min-width: 0; }
  .wbranch { display: inline-flex; align-items: center; gap: 4px; font-family: var(--mono); font-size: 10.5px; color: var(--text-dim); background: color-mix(in srgb, #57b6c9 12%, transparent); border: 1px solid color-mix(in srgb, #57b6c9 35%, transparent); border-radius: var(--radius-sm); padding: 1px 7px 1px 5px; max-width: 100%; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .wbranch > :global(svg) { color: #57b6c9; flex-shrink: 0; }
  .ic { color: var(--text-faint); display: inline-flex; flex-shrink: 0; } .ic:hover { color: var(--text); } .ic.danger:hover { color: var(--danger); }
</style>
