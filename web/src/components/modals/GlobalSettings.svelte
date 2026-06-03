<script lang="ts">
  import Modal from '../Modal.svelte';
  import Swatches from '../ui/Swatches.svelte';
  import Icon from '../ui/Icon.svelte';
  import { app } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import { confirm, closeModal } from '../../lib/modals.svelte';
  import { applyAccent, applyMonokai, DEFAULT_ACCENT } from '../../lib/theme';
  import { shellName } from '../../lib/util';
  import { toast } from '../../lib/toast.svelte';

  let { onclose }: { onclose: () => void } = $props();

  // the resolved global shell name, with sane fallbacks (ports globalShellName)
  const globalShell = () => app.shellName || (app.settings.shells && app.settings.shells[0]) || 'bash';

  // shell <select> option list, ensuring the default is present (ports shellSelectHtml)
  function shellOptions(value: string): string[] {
    const names = (app.settings.shells && app.settings.shells.length) ? app.settings.shells : ['zsh', 'bash', 'fish', 'sh'];
    const v = shellName(value);
    const list = names.slice();
    if (v && !list.includes(v)) list.unshift(v);
    return list;
  }

  let accent = $state(app.settings.accent || DEFAULT_ACCENT);
  let shell = $state(shellName(app.settings.shell) || globalShell());
  let port = $state(app.settings.port ? String(app.settings.port) : '');

  const shellList = shellOptions(shell);

  // persist settings; returns the PUT result (or null on failure), reloads
  // settings + shell name into the store. Used by both Save and Restart.
  async function persist(): Promise<{ ok: boolean; restart_needed: boolean } | null> {
    try {
      const j = await api.putSettings({ shell, accent, port: parseInt(port.trim(), 10) || 0 });
      app.settings = await api.settings();
      app.shellName = (await api.shell()).name;
      return j;
    } catch (err) {
      toast('Save settings failed: ' + (err as Error).message, 'error');
      return null;
    }
  }

  async function save() {
    const j = await persist();
    if (!j) return;
    accent === 'monokai' ? applyMonokai() : applyAccent(accent);
    closeModal();
    toast('Settings saved', 'ok');
  }

  async function restart() {
    const j = await persist();
    if (!j) return;
    const ok = await confirm({
      title: 'Restart server?',
      body: 'The orchid daemon will restart' + (j.restart_needed ? ' on the new port' : '') +
        ". Running sessions are kept (tmux); reload the page once it's back.",
      confirmLabel: 'Restart',
      danger: false,
    });
    if (!ok) return;
    try {
      const res = await api.restartDaemon();
      closeModal();
      toast('Restarting — reload ' + (res.addr || 'the page') + ' shortly', 'ok');
    } catch (err) {
      toast('Restart failed: ' + (err as Error).message, 'error');
    }
  }
</script>

<Modal title="Settings" desc="Global defaults. Projects and sessions can override these." {onclose}>
  <div class="set-scope"><Icon name="gear" size={13} /> Global</div>

  <div class="field">
    <label>Accent color<span class="fhint">UI theme</span></label>
    <Swatches value={accent} allowNone={false} onpick={(c) => (accent = c || DEFAULT_ACCENT)} />
  </div>

  <div class="field">
    <label>Default shell<span class="fhint">for new terminals</span></label>
    <select class="text" bind:value={shell}>
      {#each shellList as n}<option value={n}>{n}</option>{/each}
    </select>
  </div>

  <div class="field">
    <label>Server port<span class="fhint">restart to apply</span></label>
    <div class="port-row">
      <input class="text mono" bind:value={port} placeholder="8081" />
      <button class="btn icon" title="Restart the orchid daemon" onclick={restart}><Icon name="restart" size={17} /></button>
    </div>
  </div>

  <div class="set-soon mono">listening on {app.settings.addr || '?'}</div>

  {#snippet footer()}
    <button class="btn ghost" onclick={onclose}>Cancel</button>
    <button class="btn primary" onclick={save}>Save</button>
  {/snippet}
</Modal>
