<script lang="ts">
  import type { Session } from '../lib/types';
  import { app } from '../lib/state.svelte';
  import { setHash } from '../lib/router';
  import { agentMeta, statusMeta, shellName } from '../lib/util';
  import { colorHex } from '../lib/theme';
  import Badge from './ui/Badge.svelte';

  let { session, active }: { session: Session; active: boolean } = $props();

  const name = $derived(((session.name || '').trim()) || agentMeta(session.agent).label);
  // the command/program label: session shell choice (shell sessions) > agent cmd
  const cmd = $derived(
    session.agent === 'shell' && (session.shell || '').trim()
      ? shellName(session.shell)
      : agentMeta(session.agent).cmd
  );
  const st = $derived(statusMeta(session.status));
  const scol = $derived(colorHex(session.color));

  function open() {
    app.cur.activeId = session.id;
    setHash();
  }
</script>

<div
  class="rail-item"
  class:active
  data-id={session.id}
  style={scol ? `--scol:${scol}` : null}
  onclick={open}
  role="button"
  tabindex="0"
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); open(); } }}
>
  <span class="rail-drag" title="Drag to reorder"><Badge agent={session.agent} color={session.color} /></span>
  <div class="meta"><div class="sname">{name}</div><div class="stype">{cmd}</div></div>
  <span class="dot {st.cls}" class:pulse={st.pulse} data-status={session.id}></span>
</div>

<style>
  .rail-item { display: flex; align-items: center; gap: 9px; padding: 9px 10px; border-radius: var(--radius-sm); cursor: pointer; transition: background .1s; }
  .rail-item:hover { background: var(--surface-2); }
  .rail-item.active { background: var(--accent-bg); }
  .rail-item .meta { flex: 1; min-width: 0; }
  .rail-item .sname { font-size: 12.5px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rail-item .stype { font-family: var(--mono); font-size: 10px; color: var(--text-faint); }

  .rail-drag { display: inline-flex; cursor: grab; flex-shrink: 0; }
  .rail-drag:active { cursor: grabbing; }

  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.idle { background: var(--idle); }
  .dot.stopped { background: #3a3a42; }
  .dot.wait { background: var(--wait); }
  .dot.pulse { animation: pulse 1.3s ease-in-out infinite; }
  .dot.ok.pulse { color: var(--ok); }
  .dot.wait.pulse { color: var(--wait); }
  @keyframes pulse {
    0%   { box-shadow: 0 0 0 0 color-mix(in srgb, currentColor 65%, transparent); transform: scale(1); }
    50%  { box-shadow: 0 0 0 5px color-mix(in srgb, currentColor 10%, transparent); transform: scale(1.35); }
    100% { box-shadow: 0 0 0 8px transparent; transform: scale(1); }
  }
</style>
