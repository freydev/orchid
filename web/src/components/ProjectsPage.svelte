<script lang="ts">
  import { app } from '../lib/state.svelte';
  import { openModal } from '../lib/modals.svelte';
  import ProjectCard from './ProjectCard.svelte';
  import Icon from './ui/Icon.svelte';

  const all = $derived(app.projects.flatMap((p) => p.session_list || []));
  const running = $derived(all.filter((s) => s.status === 'running').length);
  const waiting = $derived(all.filter((s) => s.status === 'requires_action').length);
  const idle = $derived(all.filter((s) => s.status === 'idle' || s.status === 'stopped').length);
</script>

<div class="wrap page">
  <div class="proj-page">
    <div class="page-head">
      <div>
        <h1>Projects</h1>
        <div class="sub">
          {#if running}<span class="sum ok"><span class="dot ok"></span>{running} running</span>{/if}
          {#if waiting}<span class="sum wait"><span class="dot wait"></span>{waiting} waiting</span>{/if}
          {#if idle}<span class="sum"><span class="dot idle"></span>{idle} idle</span>{/if}
          {#if all.length === 0}<span>no sessions running</span>{/if}
        </div>
      </div>
      <button class="btn primary" onclick={() => openModal('newProject')}><Icon name="plus" size={15} /> New project</button>
    </div>
    {#if app.projects.length}
      <div class="proj-list">
        {#each app.projects as p (p.id)}<ProjectCard project={p} />{/each}
      </div>
    {:else}
      <div class="empty">
        <div class="big">No projects yet</div>
        <div style="margin-bottom:18px">Add a git repository to start.</div>
        <button class="btn primary" style="margin:0 auto" onclick={() => openModal('newProject')}><Icon name="plus" size={15} /> New project</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .wrap { max-width: none; margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100%; }
  .wrap.page { padding: 20px 24px 48px; }

  .proj-page { max-width: 860px; margin: 0 auto; padding: 36px 28px 80px; }
  .proj-page .page-head { margin-bottom: 22px; }
  .proj-page .sub { display: flex; align-items: center; gap: 16px; }
  .sum { display: inline-flex; align-items: center; gap: 6px; color: var(--text-dim); }
  .sum.ok { color: var(--ok); }
  .sum.wait { color: var(--wait); }
  .proj-list { display: flex; flex-direction: column; gap: 12px; }

  .page-head { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: 24px; }
  .page-head h1 { font-size: 22px; font-weight: 600; letter-spacing: -0.02em; }
  .page-head .sub { color: var(--text-dim); font-size: 13px; margin-top: 3px; }
  /* calm override */
  .page-head h1 { font-size: 20px; font-weight: 600; letter-spacing: -0.02em; }

  .empty { text-align: center; padding: 70px 20px; color: var(--text-faint); }
  .empty .big { font-size: 15px; color: var(--text-dim); margin-bottom: 6px; }

  /* status dot (activity summary) */
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.idle { background: var(--idle); }
  .dot.wait { background: var(--wait); }
</style>
