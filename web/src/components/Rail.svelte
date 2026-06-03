<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { parseBranch } from '../lib/util';
  import { openModal } from '../lib/modals.svelte';
  import Icon from './ui/Icon.svelte';
  import RailItem from './RailItem.svelte';

  // worktrees with their sessions + parsed branch label
  const groups = $derived(
    app.worktrees.map((wt) => ({
      wt,
      wb: parseBranch(wt.name),
      rows: app.sessions.filter((s) => s.worktree === wt.path),
      removing: app.removingWt.has(wt.path),
    }))
  );
</script>

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
    <div class="rail-list">
      {#if g.rows.length}
        {#each g.rows as s (s.id)}
          <RailItem session={s} active={s.id === app.cur.activeId} />
        {/each}
      {:else}
        <div class="rail-empty">no sessions</div>
      {/if}
    </div>
  </div>
{/each}

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
