<script lang="ts">
  import type { Session } from '../lib/types';
  import { app, refresh } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import { agentMeta, statusMeta, shellName } from '../lib/util';
  import { api } from '../lib/api';
  import { openModal } from '../lib/modals.svelte';
  import { terminal } from '../actions/terminal';
  import { railResize } from '../actions/railResize';
  import Rail from './Rail.svelte';
  import Badge from './ui/Badge.svelte';
  import StatusPill from './ui/StatusPill.svelte';
  import BranchLabel from './ui/BranchLabel.svelte';
  import Icon from './ui/Icon.svelte';

  // pick the active session: cur.activeId if still present, else first alive, else first
  const active = $derived<Session | null>(
    (app.cur.activeId && app.sessions.find((s) => s.id === app.cur.activeId)) ||
      app.sessions.find((s) => s.alive) ||
      app.sessions[0] ||
      null
  );

  // keep cur.activeId resolved to the picked session (mirrors app.js mountTerminal)
  $effect(() => {
    if (active && app.cur.activeId !== active.id) app.cur.activeId = active.id;
    if (!active && app.cur.activeId !== null) app.cur.activeId = null;
  });

  const name = $derived(active ? (((active.name || '').trim()) || agentMeta(active.agent).label) : '');
  const cmd = $derived(
    active && active.agent === 'shell' && (active.shell || '').trim()
      ? shellName(active.shell)
      : active ? agentMeta(active.agent).cmd : ''
  );
  // the worktree name + git branch for the active session
  const wt = $derived(active ? app.worktrees.find((w) => w.path === active.worktree) : undefined);
  const wtName = $derived(wt?.name ?? '');
  const branch = $derived((wt?.branch || '').trim());
  const scol = $derived(active?.color);

  async function onStop(id: string) { try { await api.stop(id); } finally { await refresh(); } }
  async function onRestart(id: string) { try { await api.restart(id); } finally { await refresh(); } }
</script>

<div class="focus-view">
  <div class="focus-rail"><Rail /></div>
  <div class="rail-resizer" title="Drag to resize" use:railResize></div>
  <div class="focus-term">
    {#if active}
      <div class="term" style="flex:1;min-height:0">
        <div class="term-head" style={scol ? `--scol:${scol}` : null}>
          <Badge agent={active.agent} color={active.color} />
          <div class="term-id">
            <div class="term-parent"><Icon name="branch" size={11} /><BranchLabel name={wtName} /></div>
            <div class="t-line">
              <div class="t">{name}</div>
              {#if branch}
                <span class="wt-branch" title="git branch"><Icon name="branch" size={11} />{branch}</span>
              {/if}
            </div>
          </div>
          <span class="tt">
            {#if active.command}<span class="s-cmd-tag">$ {active.command}</span>{:else}{cmd}{/if}
          </span>
          <div class="spacer"></div>
          <StatusPill status={active.status} id={active.id} />
          <button class="btn sm icon ghost" title="Session settings" onclick={() => openModal('sessionSettings', { session: active })}><Icon name="gear" size={18} /></button>
          {#if active.alive}
            <button class="btn sm icon ghost" title="Stop (keep session)" onclick={() => onStop(active.id)}><Icon name="power" size={22} /></button>
          {:else}
            <button class="btn sm icon ghost" title="Restart" onclick={() => onRestart(active.id)}><Icon name="restart" size={18} /></button>
          {/if}
          <button class="btn sm icon ghost danger" title="Kill (delete session)" onclick={() => openModal('confirmKill', { session: active })}><Icon name="x" size={22} /></button>
        </div>
        {#if active.alive}
          {#key active.id}
            <div class="pbody" style="flex:1;min-height:0;overflow:hidden;background:var(--term-bg)" use:terminal={{ id: active.id }}></div>
          {/key}
        {:else}
          <div class="pbody" style="flex:1;min-height:0;display:grid;place-items:center;background:var(--term-bg)"><span class="faint">stopped — restart to attach</span></div>
        {/if}
      </div>
    {:else}
      <div id="placeholder">no sessions — create one</div>
    {/if}
  </div>
</div>

<style>
  .focus-view { display: grid; grid-template-columns: var(--rail-w, 230px) 7px 1fr; gap: 0; padding: 20px 24px; flex: 1; min-height: 0; }
  .rail-resizer { cursor: col-resize; align-self: stretch; border-radius: var(--radius-sm); transition: background .12s; }
  .rail-resizer:hover, .rail-resizer:global(.dragging) { background: color-mix(in srgb, var(--accent) 55%, transparent); }
  .focus-rail { border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); overflow-y: auto; padding: 8px; }
  .focus-term { border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; display: flex; flex-direction: column; min-height: 0; }

  .term { display: flex; flex-direction: column; min-height: 0; background: var(--term-bg); }
  .term-head { display: flex; align-items: center; gap: 10px; padding: 9px 12px; background: var(--surface); border-bottom: 1px solid var(--border); flex-shrink: 0; }
  .term-head .term-id { min-width: 0; flex: 1; }
  .term-head .t { font-size: 12.5px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .term-head .tt { font-family: var(--mono); font-size: 11px; color: var(--text-faint); }
  .term-head .spacer { flex: 0 0 0; }
  .term-head :global(.btn.sm.icon) { width: 34px; height: 34px; }

  .term-id { min-width: 0; }
  .t-line { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .term-parent { display: flex; align-items: center; gap: 5px; min-width: 0; font-family: var(--mono); font-size: 10px; color: var(--text-faint); margin-bottom: 2px; }
  .term-parent > :global(svg) { flex-shrink: 0; }
  .term-parent :global(.b-rest) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .wt-branch { display: inline-flex; align-items: center; gap: 4px; font-family: var(--mono); font-size: 10.5px; color: var(--text-dim); background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 1px 6px 1px 4px; white-space: nowrap; flex-shrink: 0; }
  .wt-branch > :global(svg) { color: var(--text-faint); flex-shrink: 0; }

  .s-cmd-tag { font-family: var(--mono); font-size: 11px; color: var(--accent); background: var(--accent-bg); padding: 1px 7px; border-radius: var(--radius-sm); white-space: nowrap; }

  /* xterm.css hardcodes #000 on the viewport — override so the canvas matches --term-bg */
  .pbody :global(.xterm),
  .pbody :global(.xterm .xterm-viewport),
  .pbody :global(.xterm .xterm-screen) { background-color: var(--term-bg) !important; }

  #placeholder { color: var(--text-faint); font-family: var(--mono); font-size: 12px; padding: 14px 16px; }
</style>
