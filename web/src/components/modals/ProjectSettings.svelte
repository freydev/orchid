<script lang="ts">
  import Modal from '../Modal.svelte';
  import Swatches from '../ui/Swatches.svelte';
  import Icon from '../ui/Icon.svelte';
  import { app, refresh, loadProjects } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import { toast } from '../../lib/toast.svelte';
  import { shellName } from '../../lib/util';
  import type { Project, Preset } from '../../lib/types';

  let { project, onclose }: { project: Project; onclose: () => void } = $props();

  const globalShell = () => app.shellName || (app.settings.shells && app.settings.shells[0]) || 'bash';
  function shellOptions(value: string): string[] {
    const names = (app.settings.shells && app.settings.shells.length) ? app.settings.shells : ['zsh', 'bash', 'fish', 'sh'];
    const v = shellName(value);
    const list = names.slice();
    if (v && !list.includes(v)) list.unshift(v);
    return list;
  }

  let name = $state(project.name || '');
  let color = $state(project.color || '');
  let description = $state(project.description || '');
  let shell = $state(shellName(project.shell) || globalShell());
  let baseBranch = $state((project.base_branch || '').trim());

  const shellList = shellOptions(shell);

  // command-presets editor: a local array of (id, label, cmd) rows
  let commands = $state<Preset[]>(
    (project.commands || []).map((c) => ({ id: c.id || '', label: c.label || '', cmd: c.cmd || '' }))
  );
  const addCommand = () => { commands = [...commands, { id: '', label: '', cmd: '' }]; };
  const removeCommand = (i: number) => { commands = commands.filter((_, j) => j !== i); };

  // base-branch picker: (repo default) + local branches; preselect saved value
  let branches = $state<string[]>([]);
  (async () => {
    try {
      const g = await api.gitInfo(project.id);
      if (g.is_repo) branches = g.branches || [];
    } catch {}
  })();

  async function save() {
    if (!name.trim()) return;
    // drop rows with empty label or cmd
    const cmds = commands
      .map((c) => ({ id: c.id || '', label: c.label.trim(), cmd: c.cmd.trim() }))
      .filter((c) => c.label && c.cmd);
    try {
      await api.updateProject(project.id, {
        name: name.trim(),
        color,
        description: description.trim(),
        shell,
        base_branch: baseBranch.trim(),
        commands: cmds,
      });
      onclose();
      await loadProjects();
      if (app.cur.projectId) await refresh();
      toast('Project saved', 'ok');
    } catch (err) {
      toast('Save failed: ' + (err as Error).message, 'error');
    }
  }
</script>

{#snippet pathDesc()}
  <p class="mono">{project.path}</p>
{/snippet}

<Modal title="Project settings" desc={pathDesc} {onclose}>
  <div class="set-scope proj"><Icon name="folder" size={13} /> {project.name}</div>

  <div class="field">
    <label>Name</label>
    <input class="text" bind:value={name} />
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
      <select class="text mono" bind:value={baseBranch}>
        <option value="">(repo default)</option>
        {#if baseBranch && !branches.includes(baseBranch)}<option value={baseBranch}>{baseBranch}</option>{/if}
        {#each branches as b}<option value={b}>{b}</option>{/each}
      </select>
    </div>
  </div>

  <div class="field">
    <label>Command presets<span class="fhint">selectable when starting a shell</span></label>
    <div class="cmd-editor">
      {#each commands as c, i (i)}
        <div class="cmd-row">
          <input class="text cmd-label-in" placeholder="label" bind:value={c.label} />
          <input class="text mono cmd-cmd-in" placeholder="npm run dev" spellcheck="false" bind:value={c.cmd} />
          <button type="button" class="btn sm icon ghost danger" title="Remove" onclick={() => removeCommand(i)}><Icon name="x" size={18} /></button>
        </div>
      {/each}
      {#if !commands.length}
        <div class="cmd-empty">No command presets. Add one to start tmux sessions running it.</div>
      {/if}
    </div>
    <button type="button" class="btn sm cmd-add" onclick={addCommand}><Icon name="plus" size={14} /> Add command</button>
  </div>

  {#snippet footer()}
    <button class="btn ghost" onclick={onclose}>Cancel</button>
    <button class="btn primary" onclick={save}>Save</button>
  {/snippet}
</Modal>
