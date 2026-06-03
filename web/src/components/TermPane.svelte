<script lang="ts">
  import type { Session } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import { agentMeta } from '../lib/util';
  import { openModal } from '../lib/modals.svelte';
  import { terminal } from '../actions/terminal';
  import Badge from './ui/Badge.svelte';
  import StatusPill from './ui/StatusPill.svelte';
  import BranchLabel from './ui/BranchLabel.svelte';
  import Icon from './ui/Icon.svelte';

  let { session }: { session: Session } = $props();

  const name = $derived(((session.name || '').trim()) || agentMeta(session.agent).label);
  const wtName = $derived(app.worktrees.find((w) => w.path === session.worktree)?.name ?? '');
  const scol = $derived(session.color);

  function expand() {
    app.cur.view = 'terminal';
    app.cur.activeId = session.id;
    setHash();
  }
</script>

<div class="term-pane" data-pane={session.id}>
  <div class="term" style="flex:1;min-height:0">
    <div class="term-head" style={scol ? `--scol:${scol}` : null}>
      <span class="grip" title="Drag to reorder"><Icon name="grip" size={13} /></span>
      <Badge agent={session.agent} color={session.color} />
      <div class="term-id">
        <div class="term-parent"><Icon name="branch" size={11} /><BranchLabel name={wtName} /></div>
        <div class="t">{name}</div>
      </div>
      <div class="spacer"></div>
      <StatusPill status={session.status} id={session.id} />
      <button class="btn sm icon ghost" title="Session settings" onclick={() => openModal('sessionSettings', { session })}><Icon name="gear" size={16} /></button>
      <button class="btn sm icon ghost" title="Open terminal" onclick={expand}><Icon name="expand" size={16} /></button>
    </div>
    <div class="pbody" style="flex:1;min-height:0;overflow:hidden;background:var(--term-bg)" use:terminal={{ id: session.id, readonly: true, noResize: true }}></div>
  </div>
</div>

<style>
  .term-pane { border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; display: flex; flex-direction: column; height: 300px; }

  .term { display: flex; flex-direction: column; min-height: 0; background: var(--term-bg); }
  .term-head { display: flex; align-items: center; gap: 10px; padding: 9px 12px; background: var(--surface); border-bottom: 1px solid var(--border); flex-shrink: 0; }
  .term-head .term-id { min-width: 0; flex: 1; }
  .term-head .t { font-size: 12.5px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .term-head .spacer { flex: 1; }
  .term-head :global(.btn.sm.icon) { width: 34px; height: 34px; }

  .grip { display: inline-flex; align-items: center; justify-content: center; color: var(--text-faint); cursor: grab; opacity: 0; transition: opacity .12s, color .12s; flex-shrink: 0; }
  .grip:active { cursor: grabbing; }
  .grip:hover { color: var(--text-dim); }
  .term-head:hover .grip { opacity: .55; }

  .term-id { min-width: 0; }
  .term-parent { display: flex; align-items: center; gap: 5px; min-width: 0; font-family: var(--mono); font-size: 10px; color: var(--text-faint); margin-bottom: 2px; }
  .term-parent > :global(svg) { flex-shrink: 0; }
  .term-parent :global(.b-rest) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  /* xterm.css hardcodes #000 on the viewport — override so the canvas matches --term-bg */
  .pbody :global(.xterm),
  .pbody :global(.xterm .xterm-viewport),
  .pbody :global(.xterm .xterm-screen) { background-color: var(--term-bg) !important; }
</style>
