<script lang="ts">
  import type { Worktree, Session } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import { api } from '../lib/api';
  import { openModal } from '../lib/modals.svelte';
  import { sortable } from '../actions/sortable';
  import { reorderBy } from '../lib/util';
  import Icon from './ui/Icon.svelte';
  import BranchLabel from './ui/BranchLabel.svelte';
  import SessionCard from './SessionCard.svelte';

  let { wt }: { wt: Worktree } = $props();

  const rows = $derived(app.sessions.filter((s) => s.worktree === wt.path));

  // Reorder this worktree's sessions in `app.sessions` to follow `ids`, keeping
  // every other worktree's sessions in place, then persist the full id order.
  function onOrder(ids: string[]) {
    const reordered = reorderBy(rows, ids, (s) => s.id);
    let i = 0;
    app.sessions = app.sessions.map((s) =>
      s.worktree === wt.path ? reordered[i++] : s
    );
    api.orderSessions(app.sessions.map((s) => s.id));
  }
  // default: empty worktrees collapsed, non-empty open; user toggle overrides
  const open = $derived(
    app.cur.collapsed[wt.path] === undefined ? rows.length > 0 : !app.cur.collapsed[wt.path]
  );
  const removing = $derived(app.removingWt.has(wt.path));

  function toggle() {
    app.cur.collapsed[wt.path] = open; // collapse if currently open, expand if collapsed
  }
</script>

<div class="wt-group" class:removing data-wtpath={wt.path}>
  <div class="wt-group-head clickable" onclick={toggle} role="button" tabindex="0"
       onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggle(); } }}>
    <span class="grip grip-wt" title="Drag to reorder"><Icon name="grip" size={13} /></span>
    <span class="wt-chev" class:open><Icon name="chevR" size={14} /></span>
    <span class="br" title={wt.name}><Icon name="branch" size={13} /><BranchLabel name={wt.name} /></span>
    {#if (wt.branch || '').trim()}
      <span class="wt-branch" title="git branch"><Icon name="branch" size={11} />{wt.branch}</span>
    {/if}
    <span class="count">· {rows.length} session{rows.length === 1 ? '' : 's'}</span>
    <span class="spacer"></span>
    {#if removing}
      <span class="wt-removing"><span class="spinner"></span> Removing…</span>
    {:else}
      <button class="btn ghost sm" title="View diff" onclick={() => openModal('diff', { wt })}><Icon name="diff" size={14} /> Diff</button>
      <button class="btn ghost sm" onclick={() => openModal('newSession', { wt })}><Icon name="plus" size={13} /> Session</button>
      {#if !wt.is_main}
        <button class="btn ghost sm danger" title="Remove worktree" onclick={() => openModal('confirmRemoveWt', { wt })}><Icon name="x" size={14} /> Remove</button>
      {/if}
    {/if}
  </div>
  {#if open}
    {#if rows.length}
      <div class="cards" use:sortable={{ itemSel: '.s-card', handle: '.sc-grip', getId: (el) => el.dataset.id!, onOrder }}>
        {#each rows as s (s.id)}
          <SessionCard session={s} />
        {/each}
      </div>
    {:else}
      <div class="faint no-sess">no active sessions in this worktree</div>
    {/if}
  {/if}
</div>

<style>
  .wt-group { margin-bottom: 26px; }
  .wt-group.removing { opacity: .55; pointer-events: none; }
  .wt-group-head { display: flex; align-items: center; gap: 11px; margin-bottom: 11px; }
  .wt-group-head .spacer { flex: 1; align-self: center; height: 3px; margin: 0 10px; border: none; background-image: radial-gradient(circle, var(--border-2) 1.1px, transparent 1.6px); background-size: 14px 3px; background-position: center; background-repeat: repeat-x; }
  .wt-group-head.clickable { cursor: pointer; }
  .wt-group-head.clickable:hover .br { color: var(--accent); }
  .wt-chev { display: inline-flex; color: var(--text-faint); transition: transform .15s; }
  .wt-chev.open { transform: rotate(90deg); }
  /* list header: chip + full name on one wide row */
  .wt-group-head .br { display: flex; align-items: center; gap: 7px; min-width: 0; max-width: 60%; }
  .wt-group-head .br > :global(svg) { flex-shrink: 0; color: var(--text-dim); }
  .wt-group-head .br :global(.b-rest) { font-size: 13px; color: var(--text); font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .wt-group-head .count { color: var(--text-faint); font-size: 12px; white-space: nowrap; }

  .wt-group-head .grip { display: inline-flex; align-items: center; justify-content: center; color: var(--text-faint); cursor: grab; opacity: 0; transition: opacity .12s, color .12s; flex-shrink: 0; }
  .wt-group-head .grip:active { cursor: grabbing; }
  .wt-group-head .grip:hover { color: var(--text-dim); }
  .wt-group-head:hover .grip { opacity: .55; }

  .wt-branch { display: inline-flex; align-items: center; gap: 4px; font-family: var(--mono); font-size: 10.5px; color: var(--text-dim); background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 1px 6px 1px 4px; white-space: nowrap; flex-shrink: 0; }
  .wt-branch > :global(svg) { color: var(--text-faint); flex-shrink: 0; }

  .wt-removing { display: inline-flex; align-items: center; gap: 7px; font-size: 12px; color: var(--text-faint); white-space: nowrap; }

  .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 12px; }
  .faint { color: var(--text-faint); }
  .no-sess { font-size: 12.5px; padding: 4px 2px 6px; }
</style>
