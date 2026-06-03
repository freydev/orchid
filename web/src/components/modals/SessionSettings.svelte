<script lang="ts">
  import Modal from '../Modal.svelte';
  import Swatches from '../ui/Swatches.svelte';
  import Badge from '../ui/Badge.svelte';
  import { app, refresh } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import { toast } from '../../lib/toast';
  import { agentMeta, shellName } from '../../lib/util';
  import type { Session } from '../../lib/types';

  let { session, onclose }: { session: Session; onclose: () => void } = $props();

  const globalShell = () => app.shellName || (app.settings.shells && app.settings.shells[0]) || 'bash';
  function shellOptions(value: string): string[] {
    const names = (app.settings.shells && app.settings.shells.length) ? app.settings.shells : ['zsh', 'bash', 'fish', 'sh'];
    const v = shellName(value);
    const list = names.slice();
    if (v && !list.includes(v)) list.unshift(v);
    return list;
  }

  const isShell = session.agent === 'shell';
  const meta = agentMeta(session.agent);
  const sessName = (session.name || '').trim() || meta.label;

  // preselect the session's own shell, else project, else global
  const proj = app.projects.find((p) => p.id === session.project) || ({} as any);
  const curShell = shellName(session.shell) || shellName(proj.shell) || globalShell();

  let name = $state((session.name || '').trim());
  let color = $state(session.color || '');
  let shell = $state(curShell);

  const shellList = shellOptions(shell);

  async function save() {
    try {
      const body: { name: string; color: string; shell?: string } = { name: name.trim(), color };
      if (isShell) body.shell = shell;
      await api.updateSession(session.id, body);
      onclose();
      await refresh();
      toast('Session saved', 'ok');
    } catch (err) {
      toast('Save failed: ' + (err as Error).message, 'error');
    }
  }
</script>

{#snippet cmdDesc()}
  <p><span class="mono">{meta.cmd}</span></p>
{/snippet}

<Modal title="Session settings" desc={cmdDesc} {onclose}>
  <div class="set-scope sess"><Badge agent={session.agent} color={session.color} /> {sessName}</div>

  <div class="field">
    <label>Name</label>
    <input class="text" bind:value={name} placeholder={meta.label} />
  </div>
  <div class="field">
    <label>Color<span class="fhint">overrides agent color</span></label>
    <Swatches value={color} allowNone={true} noneLabel="Agent default" onpick={(c) => (color = c)} />
  </div>

  {#if isShell}
    <div class="field">
      <label>Shell<span class="fhint">for this session</span></label>
      <select class="text" bind:value={shell}>
        {#each shellList as n}<option value={n}>{n}</option>{/each}
      </select>
    </div>
    <div class="faint" style="font-size:11.5px">Shell changes apply on the next restart.</div>
  {/if}

  {#snippet footer()}
    <button class="btn ghost" onclick={onclose}>Cancel</button>
    <button class="btn primary" onclick={save}>Save</button>
  {/snippet}
</Modal>
