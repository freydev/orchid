<script lang="ts">
  import type { Project } from '../lib/types';
  import { app, refresh } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import { colorHex } from '../lib/theme';
  import { openModal } from '../lib/modals.svelte';
  import Icon from './ui/Icon.svelte';
  import BranchLabel from './ui/BranchLabel.svelte';
  import Seg from './Seg.svelte';

  let { project }: { project: Project } = $props();

  const pcol = $derived(colorHex(project.color));
  const barStyle = $derived(pcol ? `--pcol:${pcol}` : null);

  const ns = $derived(app.sessions.length);
  const nw = $derived(app.worktrees.length);
  const stopped = $derived(app.sessions.filter((s) => !s.alive).length);

  function back() {
    app.cur.projectId = null;
    setHash();
    refresh();
  }
</script>

<div class="detail-bar" style={barStyle}>
  <button class="db-back" title="Back to projects" onclick={back}><Icon name="back" size={15} /></button>
  {#if pcol}<span class="pc-dot"></span>{:else}<Icon name="folder" size={16} />{/if}
  <span class="db-name">{project.name}</span>
  {#if (project.branch || '').trim()}
    <span class="db-branch" title="current branch"><Icon name="branch" size={12} /><BranchLabel name={project.branch ?? ''} /></span>
  {/if}
  <span class="db-path mono">{project.path}</span>
  <span class="db-meta mono">{ns} {ns === 1 ? 'session' : 'sessions'} · {nw} wt</span>
  <span class="spacer"></span>
  <Seg />
  {#if stopped}
    <button class="btn sm ghost danger" title="Remove stopped sessions" onclick={() => openModal('confirmPrune')}><Icon name="trash" size={14} /> Clear stopped ({stopped})</button>
  {/if}
  <button class="btn sm icon ghost" title="Project settings" onclick={() => openModal('projectSettings', { project })}><Icon name="gear" size={19} /></button>
  <button class="btn sm" onclick={() => openModal('newWorktree')}><Icon name="branch" size={14} /> Worktree</button>
</div>

<style>
  .detail-bar { display: flex; align-items: center; gap: 12px; padding: 13px 24px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
  .db-back { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: var(--radius-sm); color: var(--text-dim); border: 1px solid var(--border-2); flex-shrink: 0; }
  .db-back:hover { color: var(--text); background: var(--surface-2); }
  .detail-bar > :global(svg) { color: var(--text-dim); flex-shrink: 0; }
  .db-name { font-size: 15px; font-weight: 600; letter-spacing: -0.01em; white-space: nowrap; }
  .db-path { font-size: 12px; color: var(--text-faint); white-space: nowrap; }
  .db-meta { font-size: 11.5px; color: var(--text-faint); white-space: nowrap; }
  .detail-bar .spacer { flex: 1; }
  .detail-bar :global(.btn.sm.icon) { width: 34px; height: 34px; }

  .db-branch { display: inline-flex; align-items: center; gap: 4px; font-size: 11.5px; color: var(--text-dim); font-weight: 500; background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 1px 7px 1px 5px; white-space: nowrap; flex-shrink: 0; }
  .db-branch > :global(svg) { color: var(--text-faint); flex-shrink: 0; }

  .pc-dot { width: 10px; height: 10px; border-radius: var(--radius-sm); flex-shrink: 0; box-shadow: inset 0 0 0 1px rgba(255,255,255,0.12); background: var(--text-faint); }

  .detail-bar[style*="--pcol"] { box-shadow: inset 3px 0 0 0 var(--pcol); background: linear-gradient(90deg, color-mix(in srgb, var(--pcol) 10%, transparent), transparent 260px), var(--surface); }
  .detail-bar[style*="--pcol"] .pc-dot { background: var(--pcol); }
</style>
