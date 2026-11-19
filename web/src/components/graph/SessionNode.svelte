<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { agentMeta } from '../../lib/util';
  import { colorHex } from '../../lib/theme';
  import Badge from '../ui/Badge.svelte';
  import StatusPill from '../ui/StatusPill.svelte';
  import Icon from '../ui/Icon.svelte';
  let { data }: { data: any } = $props();
  const p = $derived(data.project);
  const s = $derived(data.session);
  const name = $derived(((s.name || '').trim()) || agentMeta(s.agent).label);
  const scol = $derived(colorHex(s.color) || 'var(--accent)');
</script>

<div class="gnode session" style="--scol:{scol}">
  <Handle type="target" position={Position.Left} />
  <div class="head">
    <Badge agent={s.agent} color={s.color} />
    <button class="name" title="Open terminal" onclick={() => data.cb.openSessionTerminal(p, s)}>{name}</button>
    <StatusPill status={s.status} id={s.id} />
  </div>
  <div class="acts">
    <button class="ic" title="Session settings" onclick={() => data.cb.openSessionSettings(p, s)}><Icon name="gear" size={15} /></button>
    {#if s.alive}
      <button class="ic" title="Stop" onclick={() => data.cb.stopSession(s)}><Icon name="power" size={18} /></button>
    {:else}
      <button class="ic" title="Restart" onclick={() => data.cb.restartSession(s)}><Icon name="restart" size={15} /></button>
    {/if}
    <button class="ic danger" title="Kill" onclick={() => data.cb.killSession(s)}><Icon name="x" size={16} /></button>
  </div>
</div>

<style>
  /* SESSION = dark "terminal" card: term-bg, agent-color bar, mono name */
  .gnode { background: linear-gradient(90deg, color-mix(in srgb, var(--scol) 18%, transparent), transparent 200px), var(--term-bg); border: 1px solid var(--border); border-left: 3px solid var(--scol); border-radius: var(--radius-sm); padding: 8px 10px; box-sizing: border-box; width: 250px; height: 74px; display: flex; flex-direction: column; gap: 6px; }
  .head { display: flex; align-items: center; gap: 8px; }
  .name { flex: 1; font-family: var(--mono); font-size: 12px; font-weight: 600; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .name:hover { color: var(--accent); }
  .acts { display: flex; gap: 6px; justify-content: flex-end; }
  .ic { color: var(--text-faint); display: inline-flex; } .ic:hover { color: var(--text); } .ic.danger:hover { color: var(--danger); }
</style>
