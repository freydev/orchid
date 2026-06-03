<script lang="ts">
  import type { Worktree, Session } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import { api } from '../lib/api';
  import { parseBranch } from '../lib/util';
  import { openModal } from '../lib/modals.svelte';
  import { dndzone } from 'svelte-dnd-action';
  import Icon from './ui/Icon.svelte';
  import RailItem from './RailItem.svelte';

  type WtItem = Worktree & { id: string };
  // worktrees with their sessions + parsed branch label; `id` for the dndzone (keys on path)
  type Group = { id: string; wt: WtItem; wb: ReturnType<typeof parseBranch>; rows: Session[]; removing: boolean };
  const groups = $derived<Group[]>(
    app.worktrees.map((wt) => ({
      id: wt.path,
      wt: { ...wt, id: wt.path } as WtItem,
      wb: parseBranch(wt.name),
      rows: app.sessions.filter((s) => s.worktree === wt.path),
      removing: app.removingWt.has(wt.path),
    }))
  );

  // --- worktree group reorder ---
  // map each (possibly shadow) group back to its worktree by path
  const toWts = (items: Group[]): Worktree[] =>
    items.map((g) => app.worktrees.find((w) => w.path === g.id) ?? g.wt);
  function onWtConsider(e: CustomEvent<{ items: Group[] }>) {
    app.worktrees = toWts(e.detail.items);
  }
  function onWtFinalize(e: CustomEvent<{ items: Group[] }>) {
    app.worktrees = toWts(e.detail.items);
    if (app.cur.projectId) api.orderWorktrees(app.cur.projectId, app.worktrees.map((w) => w.path));
  }

  // --- per-worktree session reorder ---
  function applyOrder(wtPath: string, newRows: Session[]) {
    let i = 0;
    app.sessions = app.sessions.map((s) => (s.worktree === wtPath ? newRows[i++] : s));
  }
  function sessConsider(wtPath: string, e: CustomEvent<{ items: Session[] }>) {
    applyOrder(wtPath, e.detail.items);
  }
  function sessFinalize(wtPath: string, e: CustomEvent<{ items: Session[] }>) {
    applyOrder(wtPath, e.detail.items);
    api.orderSessions(app.sessions.map((s) => s.id));
  }
</script>

<div class="rail-groups" use:dndzone={{ items: groups, flipDurationMs: 140, type: 'rail-wt' }} onconsider={onWtConsider} onfinalize={onWtFinalize}>
{#each groups as g (g.id)}
  <div class="rail-group" class:removing={g.removing} data-wtpath={g.wt.path}>
    {#if g.wb.tag}
      <div class="rail-head" title={g.wt.name}>
        <div class="rail-head-top">
          <span class="rail-drag grip-wt" title="Drag worktree to reorder"><Icon name="branch" size={11} /></span>
          <span class="rail-wt-tag">{g.wb.tag}</span>
          <span class="spacer"></span>
          {#if g.removing}
            <span class="spinner" title="Removing worktree…"></span>
          {:else}
            <button class="rail-icon" title="View diff" onclick={() => openModal('diff', { wt: g.wt })}><Icon name="diff" size={13} /></button>
            <button class="rail-add" title="New session in this worktree" onclick={() => openModal('newSession', { wt: g.wt })}>+</button>
          {/if}
        </div>
        <div class="rail-wt-name">{g.wb.rest}</div>
      </div>
    {:else}
      <div class="rail-head" title={g.wt.name}>
        <div class="rail-head-top">
          <span class="rail-drag grip-wt" title="Drag worktree to reorder"><Icon name="branch" size={11} /></span>
          <span class="rail-wt-name-inline">{g.wb.rest}</span>
          {#if g.removing}
            <span class="spinner" title="Removing worktree…"></span>
          {:else}
            <button class="rail-icon" title="View diff" onclick={() => openModal('diff', { wt: g.wt })}><Icon name="diff" size={13} /></button>
            <button class="rail-add" title="New session in this worktree" onclick={() => openModal('newSession', { wt: g.wt })}>+</button>
          {/if}
        </div>
      </div>
    {/if}
    {#if g.rows.length}
      <div class="rail-list"
           use:dndzone={{ items: g.rows, flipDurationMs: 140, type: 'rail-sess' }}
           onconsider={(e) => sessConsider(g.wt.path, e)}
           onfinalize={(e) => sessFinalize(g.wt.path, e)}>
        {#each g.rows as s (s.id)}
          <RailItem session={s} active={s.id === app.cur.activeId} />
        {/each}
      </div>
    {:else}
      <div class="rail-list"><div class="rail-empty">no sessions</div></div>
    {/if}
  </div>
{/each}
</div>

<style>
  .rail-group { margin-bottom: 4px; }
  .rail-group.removing { opacity: .55; pointer-events: none; }

  .rail-head { display: flex; flex-direction: column; gap: 4px; padding: 13px 8px 6px 10px; }
  .rail-head-top { display: flex; align-items: center; gap: 6px; }
  .rail-head-top > :global(svg) { color: var(--text-faint); flex-shrink: 0; }
  .rail-head-top .spacer { flex: 1; }

  .rail-drag { display: inline-flex; cursor: grab; flex-shrink: 0; color: var(--text-faint); transition: color .12s; }
  .rail-drag:active { cursor: grabbing; }
  .rail-head:hover .rail-drag { color: var(--text-dim); }

  .rail-wt-tag { font-family: var(--mono); font-size: 9.5px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--accent); background: var(--accent-bg); padding: 1px 5px; border-radius: var(--radius-sm); white-space: nowrap; }
  .rail-wt-name { font-family: var(--mono); font-size: 11.5px; line-height: 1.35; color: var(--text); font-weight: 600; word-break: break-word; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
  .rail-wt-name-inline { flex: 1; min-width: 0; font-family: var(--mono); font-size: 11.5px; color: var(--text); font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .rail-icon { color: var(--text-faint); display: inline-flex; padding: 0 4px; flex-shrink: 0; }
  .rail-icon:hover { color: var(--accent); }
  .rail-add { color: var(--text-faint); font-size: 16px; line-height: 1; padding: 0 4px; border-radius: var(--radius-sm); flex-shrink: 0; }
  .rail-add:hover { color: var(--accent); }
  .rail-list { display: block; }
  .rail-empty { font-size: 11.5px; color: var(--text-faint); padding: 2px 10px 6px; }
</style>
