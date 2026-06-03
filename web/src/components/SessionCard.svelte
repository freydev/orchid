<script lang="ts">
  import type { Session } from '../lib/types';
  import { app, refresh } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import { agentMeta, statusMeta, shellName } from '../lib/util';
  import { colorHex } from '../lib/theme';
  import { api } from '../lib/api';
  import { openModal } from '../lib/modals.svelte';
  import { terminal } from '../actions/terminal';
  import Badge from './ui/Badge.svelte';
  import StatusPill from './ui/StatusPill.svelte';
  import Icon from './ui/Icon.svelte';

  let { session }: { session: Session } = $props();

  const name = $derived(((session.name || '').trim()) || agentMeta(session.agent).label);
  // the command/program label: preset cmd > session shell choice > agent cmd
  const cmd = $derived(
    session.agent === 'shell' && (session.shell || '').trim()
      ? shellName(session.shell)
      : agentMeta(session.agent).cmd
  );
  const scol = $derived(colorHex(session.color));
  // the git branch of this session's worktree (empty when unknown)
  const branch = $derived((app.worktrees.find((w) => w.path === session.worktree)?.branch || '').trim());
  // worktree label fallback when there's no git branch chip
  const wtLabel = $derived(app.worktrees.find((w) => w.path === session.worktree)?.name ?? '');

  // click anywhere on the card (except buttons / grip) opens the Terminal for it
  function openCard(e: MouseEvent) {
    const t = e.target as HTMLElement;
    if (t.closest('button') || t.closest('.sc-grip')) return;
    app.cur.view = 'terminal';
    app.cur.activeId = session.id;
    setHash();
  }
  async function onStop() { try { await api.stop(session.id); } finally { await refresh(); } }
  async function onRestart() { try { await api.restart(session.id); } finally { await refresh(); } }
</script>

<div
  class="s-card"
  data-id={session.id}
  style={scol ? `--scol:${scol}` : null}
  onclick={openCard}
  role="button"
  tabindex="0"
  onkeydown={(e) => { if (e.key === 'Enter') { app.cur.view = 'terminal'; app.cur.activeId = session.id; setHash(); } }}
>
  <div class="head">
    <span class="sc-grip" title="Drag to reorder"><Badge agent={session.agent} color={session.color} lg /></span>
    <div class="meta">
      <div class="sname">{name}</div>
      <div class="stype">{#if session.command}<span class="s-cmd-tag">$ {session.command}</span>{:else}{cmd}{/if}</div>
    </div>
    <StatusPill status={session.status} id={session.id} />
  </div>
  {#if session.alive}
    <div class="preview pbody" use:terminal={{ id: session.id, readonly: true, noResize: true }}></div>
  {:else}
    <div class="preview stopped"><span class="faint">stopped — restart to attach</span></div>
  {/if}
  <div class="foot">
    {#if branch}
      <span class="wt-branch" title="git branch"><Icon name="branch" size={11} />{branch}</span>
    {:else}
      <span class="up">{wtLabel}</span>
    {/if}
    <div class="acts">
      <button class="btn sm icon ghost" title="Session settings" onclick={() => openModal('sessionSettings', { session })}><Icon name="gear" size={18} /></button>
      {#if session.alive}
        <button class="btn sm icon ghost" title="Stop (keep session)" onclick={onStop}><Icon name="power" size={22} /></button>
      {:else}
        <button class="btn sm icon ghost" title="Restart" onclick={onRestart}><Icon name="restart" size={18} /></button>
      {/if}
      <button class="btn sm icon ghost danger" title="Kill (delete session)" onclick={() => openModal('confirmKill', { session })}><Icon name="x" size={22} /></button>
    </div>
  </div>
</div>

<style>
  .s-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-sm); overflow: hidden; cursor: pointer; transition: border-color .12s; display: flex; flex-direction: column; }
  .s-card:hover { border-color: var(--border-2); }
  .s-card[style*="--scol"] { box-shadow: inset 3px 0 0 0 var(--scol); }
  .s-card[style*="--scol"] .head { background: linear-gradient(90deg, color-mix(in srgb, var(--scol) 11%, transparent), transparent 200px); }
  .s-card .head { display: flex; align-items: center; gap: 11px; padding: 12px 13px; }
  .s-card .head .sc-grip { display: inline-flex; cursor: grab; flex-shrink: 0; }
  .s-card .head .sc-grip:active { cursor: grabbing; }
  .s-card .head .meta { flex: 1; min-width: 0; }
  .s-card .head .sname { font-size: 13px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .s-card .head .stype { font-size: 10.5px; color: var(--text-faint); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

  .s-card .preview { position: relative; height: 108px; overflow: hidden; background: var(--term-bg); border-top: 1px solid var(--border); pointer-events: none; }
  .s-card .preview.stopped { display: grid; place-items: center; font-size: 11.5px; padding: 0 12px; text-align: center; }
  .s-card .preview::after { content: ""; position: absolute; inset: 0; pointer-events: none; z-index: 1; box-shadow: inset 0 0 22px 9px var(--term-bg); }

  .s-card .foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 12px; border-top: 1px solid var(--border); min-height: 44px; }
  .s-card .foot .up { font-size: 10.5px; color: var(--text-faint); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .s-card .acts { display: flex; gap: 2px; opacity: 0; transition: opacity .12s; flex-shrink: 0; }
  .s-card:hover .acts { opacity: 1; }
  .s-card .acts :global(.btn.sm.icon) { width: 32px; height: 32px; }

  .s-cmd-tag { font-family: var(--mono); font-size: 11px; color: var(--accent); background: var(--accent-bg); padding: 1px 7px; border-radius: var(--radius-sm); white-space: nowrap; }

  .wt-branch { display: inline-flex; align-items: center; gap: 4px; font-family: var(--mono); font-size: 10.5px; color: var(--text-dim); background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 1px 6px 1px 4px; white-space: nowrap; flex-shrink: 0; }
  .wt-branch > :global(svg) { color: var(--text-faint); flex-shrink: 0; }

  .faint { color: var(--text-faint); }

  /* xterm.css hardcodes #000 on the viewport — override so the preview matches --term-bg */
  .preview :global(.xterm),
  .preview :global(.xterm .xterm-viewport),
  .preview :global(.xterm .xterm-screen) { background-color: var(--term-bg) !important; }
</style>
