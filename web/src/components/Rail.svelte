<script lang="ts">
  import type { Worktree, Session, Group as SessGroup } from '../lib/types';
  import { app, refresh } from '../lib/state.svelte';
  import { confirm } from '../lib/modals.svelte';
  import { toast } from '../lib/toast.svelte';
  import { escHtml } from '../lib/util';
  import { api } from '../lib/api';
  import { parseBranch, reorderBy } from '../lib/util';
  import { openModal } from '../lib/modals.svelte';
  import { sortable } from '../actions/sortable';
  import Icon from './ui/Icon.svelte';
  import RailItem from './RailItem.svelte';

  type Sub = { g: SessGroup; rows: Session[]; open: boolean };
  type Group = { wt: Worktree; wb: ReturnType<typeof parseBranch>; rows: Session[]; removing: boolean; subs: Sub[]; loose: Session[] };
  const groups = $derived<Group[]>(
    app.worktrees.map((wt) => {
      const rows = app.sessions.filter((s) => s.worktree === wt.path);
      const gs = wt.groups ?? [];
      return {
        wt,
        wb: parseBranch(wt.name),
        rows,
        removing: app.removingWt.has(wt.path),
        subs: gs.map((g) => ({ g, rows: rows.filter((s) => s.group === g.id), open: !app.cur.collapsed['g:' + g.id] })),
        // sessions whose group is unset or no longer exists
        loose: rows.filter((s) => !gs.some((g) => g.id === s.group)),
      };
    })
  );

  const plural = (n: number) => n + ' session' + (n === 1 ? '' : 's');

  async function removeWorktree(wt: Worktree) {
    const pid = app.cur.projectId;
    if (!pid || wt.is_main || app.removingWt.has(wt.path)) return;
    const n = app.sessions.filter((s) => s.worktree === wt.path).length;
    const ok = await confirm({
      title: 'Remove worktree?',
      body: `Remove <b>${escHtml(wt.branch)}</b>?` + (n ? ` This kills its <b>${plural(n)}</b>,` : ' This') +
        ` removes the worktree and deletes the branch, including uncommitted changes. This cannot be undone.`,
      confirmLabel: 'Remove',
    });
    if (!ok) return;
    app.removingWt = new Set([...app.removingWt, wt.path]);
    try {
      await api.removeWorktree(pid, wt.path);
      if (app.cur.activeId && !app.sessions.some((s) => s.id === app.cur.activeId && s.worktree !== wt.path)) app.cur.activeId = null;
      toast('Removed ' + wt.branch, 'ok');
    } catch (err) {
      toast('Remove failed: ' + (err as Error).message, 'error');
    } finally {
      const d = new Set(app.removingWt); d.delete(wt.path); app.removingWt = d;
      await refresh();
    }
  }
  function toggle(gid: string) { app.cur.collapsed['g:' + gid] = !app.cur.collapsed['g:' + gid]; }

  // drag a session into another list of the same worktree -> change its group
  async function moveTo(id: string, to: HTMLElement) {
    const gid = to.dataset.group ?? '';
    const s = app.sessions.find((x) => x.id === id);
    if (!s || (s.group ?? '') === gid) return;
    s.group = gid; // optimistic
    try { await api.updateSession(id, { group: gid }); }
    catch (err) { toast('Move failed: ' + (err as Error).message, 'error'); }
    await refresh();
  }

  async function clearGroup(sub: Sub) {
    const pid = app.cur.projectId;
    if (!pid) return;
    if (!sub.rows.length) { toast('Group is already empty'); return; }
    const ok = await confirm({
      title: 'Kill group sessions?',
      body: `Kill <b>${plural(sub.rows.length)}</b> in <b>${escHtml(sub.g.name)}</b>? Their tmux sessions and processes are terminated. The group itself stays.`,
      confirmLabel: 'Kill all',
    });
    if (!ok) return;
    toast('Killing ' + plural(sub.rows.length) + '…');
    const ids = sub.rows.map((s) => s.id);
    try {
      const r = await api.clearGroup(pid, sub.g.id);
      if (app.cur.activeId && ids.includes(app.cur.activeId)) app.cur.activeId = null;
      toast('Killed ' + plural(r.killed), 'ok');
    } catch (err) {
      toast('Clear failed: ' + (err as Error).message, 'error');
    }
    await refresh();
  }

  async function deleteGroup(sub: Sub) {
    const pid = app.cur.projectId;
    if (!pid) return;
    const n = sub.rows.length;
    const ok = await confirm({
      title: 'Delete group?',
      body: `Delete group <b>${escHtml(sub.g.name)}</b>?` + (n ? ` Its ${plural(n)} stay open and move to Ungrouped.` : ''),
      confirmLabel: 'Delete',
    });
    if (!ok) return;
    try { await api.removeGroup(pid, sub.g.id); }
    catch (err) { toast('Delete group failed: ' + (err as Error).message, 'error'); }
    await refresh();
  }

  // --- worktree group reorder ---
  function onWtOrder(ids: string[]) {
    app.worktrees = reorderBy(app.worktrees, ids, (w) => w.path);
    if (app.cur.projectId) api.orderWorktrees(app.cur.projectId, ids);
  }

  // --- per-worktree session reorder (block cross-worktree moves) ---
  function onSessOrder(wtPath: string, ids: string[]) {
    const rows = app.sessions.filter((s) => s.worktree === wtPath);
    const reordered = reorderBy(rows, ids, (s) => s.id);
    let i = 0;
    app.sessions = app.sessions.map((s) => (s.worktree === wtPath ? reordered[i++] : s));
    api.orderSessions(app.sessions.map((s) => s.id));
  }
</script>

{#snippet list(wt: Worktree, gid: string, rows: Session[], empty: string)}
  <!-- one droppable list; lists of a worktree share a Sortable group so sessions move between its groups -->
  <div class="rail-list" data-group={gid}
       use:sortable={{ itemSel: '.rail-item', handle: '.rail-drag', group: 'wt:' + wt.path, getId: (el) => el.dataset.id!,
                       onOrder: (ids) => onSessOrder(wt.path, ids), onTransfer: moveTo }}>
    {#each rows as s (s.id)}
      <RailItem session={s} active={s.id === app.cur.activeId} />
    {/each}
    {#if !rows.length}<div class="rail-empty">{empty}</div>{/if}
  </div>
{/snippet}

<div class="rail-groups" use:sortable={{ itemSel: '.rail-group', handle: '.grip-wt', getId: (el) => el.dataset.wtpath!, onOrder: onWtOrder }}>
{#each groups as g (g.wt.path)}
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
            <button class="rail-icon" title="New group in this worktree" onclick={() => openModal('groupName', { wt: g.wt.path })}><Icon name="layers" size={13} /></button>
            <button class="rail-add" title="New session in this worktree" onclick={() => openModal('newSession', { wt: g.wt.path })}>+</button>
            {#if !g.wt.is_main}
              <button class="rail-icon danger" title="Remove worktree" onclick={() => removeWorktree(g.wt)}><Icon name="x" size={13} /></button>
            {/if}
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
            <button class="rail-icon" title="New group in this worktree" onclick={() => openModal('groupName', { wt: g.wt.path })}><Icon name="layers" size={13} /></button>
            <button class="rail-add" title="New session in this worktree" onclick={() => openModal('newSession', { wt: g.wt.path })}>+</button>
            {#if !g.wt.is_main}
              <button class="rail-icon danger" title="Remove worktree" onclick={() => removeWorktree(g.wt)}><Icon name="x" size={13} /></button>
            {/if}
          {/if}
        </div>
      </div>
    {/if}
    {#if !g.subs.length}
      {@render list(g.wt, '', g.rows, 'no sessions')}
    {:else}
      {#each g.subs as sub (sub.g.id)}
        <div class="rail-sub" class:collapsed={!sub.open}>
          <div class="rail-sub-head" title={sub.g.name}>
            <button class="rail-sub-toggle" onclick={() => toggle(sub.g.id)}>
              <span class="rail-caret"><Icon name="chevR" size={11} /></span>
              <span class="rail-sub-name">{sub.g.name}</span>
              <span class="rail-sub-n">{sub.rows.length}</span>
            </button>
            <span class="rail-sub-acts">
              <button class="rail-icon" title="Rename group" onclick={() => openModal('groupName', { group: sub.g })}><Icon name="edit" size={12} /></button>
              <button class="rail-icon" title="Kill all sessions in group" onclick={() => clearGroup(sub)}><Icon name="trash" size={12} /></button>
              <button class="rail-icon" title="Delete group" onclick={() => deleteGroup(sub)}><Icon name="x" size={12} /></button>
            </span>
            <button class="rail-add" title="New session in this group" onclick={() => openModal('newSession', { wt: g.wt.path, group: sub.g.id })}>+</button>
          </div>
          {#if sub.open}{@render list(g.wt, sub.g.id, sub.rows, 'empty')}{/if}
        </div>
      {/each}
      <div class="rail-sub ungrouped">
        <div class="rail-sub-head plain"><span class="rail-sub-name">Ungrouped</span><span class="rail-sub-n">{g.loose.length}</span></div>
        {@render list(g.wt, '', g.loose, '—')}
      </div>
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
  .rail-icon.danger:hover { color: var(--danger, #e5534b); }
  .rail-add { color: var(--text-faint); font-size: 16px; line-height: 1; padding: 0 4px; border-radius: var(--radius-sm); flex-shrink: 0; }
  .rail-add:hover { color: var(--accent); }
  .rail-list { display: block; }
  .rail-empty { font-size: 11.5px; color: var(--text-faint); padding: 2px 10px 6px; }

  /* session groups inside a worktree */
  .rail-sub { margin: 2px 0 2px 6px; border-left: 1px solid var(--border); padding-left: 4px; }
  .rail-sub.ungrouped { border-left-color: transparent; }
  .rail-sub-head { display: flex; align-items: center; gap: 4px; padding: 3px 4px; border-radius: var(--radius-sm); font-size: 11.5px; color: var(--text-dim); }
  .rail-sub-head:not(.plain):hover { background: var(--surface-2); }
  .rail-sub-head.plain { padding: 5px 4px; }
  .rail-sub-toggle { flex: 1; min-width: 0; display: flex; align-items: center; gap: 5px; color: inherit; font: inherit; text-align: left; padding: 2px 0; }
  .rail-caret { display: inline-flex; color: var(--text-faint); transform: rotate(90deg); transition: transform .12s; }
  .rail-sub.collapsed .rail-caret { transform: none; }
  .rail-sub-name { flex: 1; min-width: 0; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rail-sub-n { font-family: var(--mono); font-size: 10px; color: var(--text-faint); }
  .rail-sub-acts { display: none; align-items: center; }
  .rail-sub-head:hover .rail-sub-acts { display: inline-flex; }
</style>
