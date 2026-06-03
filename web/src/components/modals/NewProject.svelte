<script lang="ts">
  import Modal from '../Modal.svelte';
  import Swatches from '../ui/Swatches.svelte';
  import { app, refresh } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import { toast } from '../../lib/toast';
  import { shellName } from '../../lib/util';

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

  let name = $state('');
  let path = $state('');
  let color = $state('');
  let description = $state('');
  let shell = $state(globalShell());
  let baseBranch = $state('');

  const shellList = shellOptions(shell);

  async function create() {
    try {
      await api.addProject({
        name: name.trim(),
        path: path.trim(),
        color,
        description: description.trim(),
        shell,
        base_branch: baseBranch.trim(),
      });
      onclose();
      await refresh();
      toast('Project added', 'ok');
    } catch (err) {
      toast((err as Error).message, 'error');
    }
  }
</script>

<Modal title="New project" desc="A git repository directory (must contain .git)." {onclose}>
  <div class="field">
    <label>Name (optional)</label>
    <input class="text" bind:value={name} placeholder="my-service" />
  </div>
  <div class="field">
    <label>Path</label>
    <input class="text mono" bind:value={path} placeholder="~/code/my-service" />
  </div>
  <div class="field">
    <label>Color</label>
    <Swatches value={color} allowNone={true} noneLabel="No color" onpick={(c) => (color = c)} />
  </div>
  <div class="field">
    <label>Description</label>
    <textarea class="text area" rows="2" bind:value={description} placeholder="What is this project?"></textarea>
  </div>
  <div class="frow">
    <div class="field">
      <label>Shell<span class="fhint">default for sessions</span></label>
      <select class="text" bind:value={shell}>
        {#each shellList as n}<option value={n}>{n}</option>{/each}
      </select>
    </div>
    <div class="field">
      <label>Base branch<span class="fhint">for new worktrees</span></label>
      <input class="text mono" bind:value={baseBranch} placeholder="main" />
    </div>
  </div>

  {#snippet footer()}
    <button class="btn ghost" onclick={onclose}>Cancel</button>
    <button class="btn primary" onclick={create}>Create</button>
  {/snippet}
</Modal>
