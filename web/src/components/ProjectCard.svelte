<script lang="ts">
  import type { Project } from '../lib/types';
  import { app, refresh } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import { colorHex } from '../lib/theme';
  import { agentMeta, statusMeta, parseBranch, STATUS_RANK, escHtml } from '../lib/util';
  import { openModal, confirm } from '../lib/modals.svelte';
  import { api } from '../lib/api';
  import { toast } from '../lib/toast.svelte';
  import Icon from './ui/Icon.svelte';
  import Badge from './ui/Badge.svelte';
  import BranchLabel from './ui/BranchLabel.svelte';

  let { project }: { project: Project } = $props();

  const pcol = $derived(colorHex(project.color));
  const cardStyle = $derived(pcol ? `--pcol:${pcol}` : null);

  const list = $derived(project.session_list || []);
  // running first, then requires_action / idle / stopped; keep order within a rank
  const sorted = $derived(
    list.slice().sort((a, b) => (STATUS_RANK[a.status] ?? 9) - (STATUS_RANK[b.status] ?? 9))
  );
  const show = $derived(sorted.slice(0, 5));
  const more = $derived(list.length - show.length);

  function chipName(c: { agent: string; name: string }) {
    return (c.name || '').trim() || agentMeta(c.agent).label;
  }

  function openProject() {
    app.cur = { projectId: project.id, activeId: null, collapsed: {} };
    setHash();
    refresh();
  }

  function onCardClick(e: MouseEvent) {
    const t = e.target as HTMLElement;
    if (t.closest('button') || t.closest('.grip')) return;
    openProject();
  }

  function onGear(e: MouseEvent) {
    e.stopPropagation();
    openModal('projectSettings', { project });
  }

  async function onDelete(e: MouseEvent) {
    e.stopPropagation();
    const ok = await confirm({
      title: 'Delete project',
      body: `Remove <b>${escHtml(project.name)}</b> from Orchid? Git worktrees and files on disk are left untouched.`,
      confirmLabel: 'Delete',
    });
    if (!ok) return;
    try {
      await api.removeProject(project.id);
      if (app.cur.projectId === project.id) {
        app.cur = { projectId: null, activeId: null, collapsed: {} };
        setHash();
      }
      await refresh();
      toast('Project removed', 'ok');
    } catch (err) {
      toast('Delete failed: ' + (err as Error).message, 'error');
    }
  }
</script>

<div
  class="proj-card"
  data-pid={project.id}
  style={cardStyle}
  role="button"
  tabindex="0"
  onclick={onCardClick}
  onkeydown={(e) => { if (e.key === 'Enter') openProject(); }}
>
  <div class="pc-head">
    <span class="grip" title="Drag to reorder"><Icon name="grip" size={14} /></span>
    {#if pcol}<span class="pc-dot"></span>{:else}<Icon name="folder" size={18} />{/if}
    <div class="pc-id">
      <div class="pc-name">
        {project.name}
        {#if (project.branch || '').trim()}
          <span class="pc-branch" title="current branch"><Icon name="branch" size={12} /><BranchLabel name={project.branch ?? ''} /></span>
        {/if}
        {#if project.problem}
          <span class="pc-problem" title="Project path problem: {project.problem}"><Icon name="warn" size={12} /> {project.problem}</span>
        {/if}
      </div>
      <div class="pc-path">{project.path}</div>
      {#if (project.description || '').trim()}<div class="pc-desc">{project.description}</div>{/if}
    </div>
    <div class="pc-meta">
      <span class="stat" title="worktrees"><Icon name="layers" size={14} /> {project.worktrees}</span>
      <span class="stat" title="sessions"><Icon name="term" size={14} /> {project.sessions}</span>
      <button class="pc-gear" title="Project settings" onclick={onGear}><Icon name="gear" size={18} /></button>
      <button class="btn sm icon ghost danger proj-del" title="Delete project" onclick={onDelete}><Icon name="x" size={22} /></button>
      <Icon name="chevR" size={16} />
    </div>
  </div>

  {#if list.length}
    <div class="pc-sessions">
      {#each show as c}
        {@const st = statusMeta(c.status)}
        <div class="pc-chip">
          <Badge agent={c.agent} color={c.color ?? null} />
          <span class="cname">{chipName(c)}</span>
          <span class="dot {st.cls}" class:pulse={st.pulse}></span>
          <span class="cwt mono">{parseBranch(c.worktree || '').rest}</span>
        </div>
      {/each}
      {#if more > 0}<div class="pc-more">+{more} more</div>{/if}
    </div>
    <!-- mobile: just the status dots in a row, no session info -->
    <div class="pc-statuses">
      <span class="pc-statuses-label">{list.length} {list.length === 1 ? 'session' : 'sessions'}</span>
      {#each sorted as c}
        {@const st = statusMeta(c.status)}
        <span class="dot {st.cls}" class:pulse={st.pulse} title={chipName(c)}></span>
      {/each}
    </div>
  {:else}
    <div class="pc-empty">No sessions yet — open to start an agent or shell</div>
  {/if}
</div>

<style>
  /* ---------- project card ---------- */
  .proj-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); cursor: pointer; transition: border-color .12s, background .12s; overflow: hidden; }
  .proj-card:hover { border-color: var(--border-2); }
  .pc-head { display: grid; grid-template-columns: auto auto 1fr auto; align-items: center; gap: 13px; padding: 15px 18px 13px; }
  .pc-head > :global(svg) { color: var(--text-dim); }
  .pc-id { min-width: 0; }
  .pc-name { font-size: 16px; font-weight: 600; letter-spacing: -0.01em; }
  .pc-path { font-family: var(--mono); font-size: 12px; color: var(--text-faint); margin-top: 2px; }
  .pc-meta { display: flex; align-items: center; gap: 16px; color: var(--text-dim); font-size: 12px; }
  .pc-meta .stat { display: flex; align-items: center; gap: 6px; }
  .pc-meta > :global(svg) { color: var(--text-faint); }
  .pc-sessions { display: flex; flex-wrap: wrap; gap: 8px; padding: 0 18px 16px; }
  .pc-chip { display: flex; align-items: center; gap: 8px; flex-shrink: 0; max-width: 100%; padding: 6px 11px 6px 7px; background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .pc-chip .cname { font-size: 12.5px; font-weight: 500; white-space: nowrap; }
  .pc-chip .cwt { font-size: 11px; color: var(--text-faint); border-left: 1px solid var(--border-2); margin-left: 1px; padding-left: 8px; white-space: nowrap; }
  .pc-more { display: flex; align-items: center; font-size: 12px; color: var(--text-faint); padding: 0 4px; }
  .pc-statuses { display: none; } /* mobile-only status-dot row */
  .pc-statuses-label { font-size: 11px; font-weight: 600; color: var(--text-faint); letter-spacing: 0.02em; margin-right: 4px; }
  .pc-empty { padding: 0 18px 16px; font-size: 12.5px; color: var(--text-faint); }
  .proj-card .proj-del { opacity: 0; transition: opacity .12s; }
  .proj-card .proj-del.btn.sm.icon { width: 36px; height: 36px; }
  .proj-card:hover .proj-del { opacity: 1; }

  /* drag handle reveal on hover */
  .grip { display: inline-flex; align-items: center; justify-content: center; color: var(--text-faint); cursor: grab; opacity: 0; transition: opacity .12s, color .12s; flex-shrink: 0; }
  .grip:active { cursor: grabbing; }
  .grip:hover { color: var(--text-dim); }
  .proj-card:hover .grip { opacity: .55; }

  /* status dot (chip) */
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.idle { background: var(--idle); }
  .dot.stopped { background: #3a3a42; }
  .dot.wait { background: var(--wait); }
  /* composited-only (transform/opacity): box-shadow pulse repainted every frame */
  .dot.pulse { position: relative; animation: pulse 1.3s ease-in-out infinite; }
  .dot.ok.pulse { color: var(--ok); }
  .dot.wait.pulse { color: var(--wait); }
  .dot.pulse::after {
    content: ''; position: absolute; inset: 0; border-radius: 50%;
    background: currentColor; animation: pulse-ring 1.3s ease-out infinite;
  }
  @keyframes pulse { 50% { transform: scale(1.35); } }
  @keyframes pulse-ring {
    0%   { transform: scale(1); opacity: .65; }
    100% { transform: scale(3); opacity: 0; }
  }

  /* project color marker + per-card gear */
  .pc-dot { width: 10px; height: 10px; border-radius: var(--radius-sm); flex-shrink: 0; box-shadow: inset 0 0 0 1px rgba(255,255,255,0.12); background: var(--text-faint); }
  .pc-desc { font-size: 12.5px; color: var(--text-dim); margin-top: 5px; line-height: 1.45; }
  .pc-name { display: flex; align-items: center; gap: 9px; flex-wrap: wrap; }
  .pc-branch { display: inline-flex; align-items: center; gap: 4px; font-size: 11.5px; color: var(--text-dim); font-weight: 500; background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 1px 7px 1px 5px; white-space: nowrap; flex-shrink: 0; }
  .pc-branch > :global(svg) { color: var(--text-faint); flex-shrink: 0; }
  .pc-problem { display: inline-flex; align-items: center; gap: 4px; font-size: 11px; font-weight: 600; color: var(--danger); background: color-mix(in srgb, var(--danger) 12%, transparent); border: 1px solid color-mix(in srgb, var(--danger) 38%, transparent); border-radius: var(--radius-sm); padding: 1px 7px 1px 5px; white-space: nowrap; flex-shrink: 0; }
  .pc-problem > :global(svg) { color: var(--danger); flex-shrink: 0; }
  .pc-gear { display: inline-flex; color: var(--text-faint); opacity: 0; transition: opacity .12s, color .12s; }
  .proj-card:hover .pc-gear { opacity: 1; }
  .pc-gear:hover { color: var(--text); }

  /* proj-card left accent bar */
  .proj-card { position: relative; }
  .proj-card::before { content: ""; position: absolute; left: 0; top: 0; bottom: 0; width: 0; background: var(--accent); transition: width .12s; z-index: 1; }
  .proj-card:hover::before { width: 3px; }

  /* project accent color: left bar + faint tint on card */
  .proj-card[style*="--pcol"] { border-left: 3px solid var(--pcol); background: linear-gradient(90deg, color-mix(in srgb, var(--pcol) 9%, transparent), transparent 200px), var(--surface); }
  .proj-card[style*="--pcol"]:hover { background: linear-gradient(90deg, color-mix(in srgb, var(--pcol) 14%, transparent), transparent 240px), var(--surface-2); }
  .proj-card[style*="--pcol"] .pc-dot { background: var(--pcol); }
  .proj-card[style*="--pcol"]::before { background: var(--pcol); }

  @media (max-width: 640px) {
    .pc-head { gap: 10px; padding: 13px 13px 11px; }
    .pc-meta { gap: 11px; }
    .pc-meta .stat { display: none; }
    .pc-sessions { display: none; }            /* hide session info chips */
    .pc-statuses { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 0 13px; padding: 11px 0 13px; border-top: 1px solid var(--border); }
  }
</style>
