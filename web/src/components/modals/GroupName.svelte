<script lang="ts">
  import Modal from '../Modal.svelte';
  import { app, refresh } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import { toast } from '../../lib/toast.svelte';
  import type { Group } from '../../lib/types';

  // create (wt set) or rename (group set) a session group
  let { wt, group, onclose }: { wt?: string; group?: Group; onclose: () => void } = $props();

  let name = $state(group?.name ?? '');
  let input: HTMLInputElement;
  $effect(() => { input?.focus(); input?.select(); });

  async function save() {
    const n = name.trim();
    const pid = app.cur.projectId;
    if (!n || !pid) return;
    try {
      if (group) { if (n !== group.name) await api.renameGroup(pid, group.id, n); }
      else await api.addGroup(pid, wt!, n);
      onclose();
      await refresh();
    } catch (err) {
      toast((group ? 'Rename failed: ' : 'Create group failed: ') + (err as Error).message, 'error');
    }
  }
</script>

<Modal title={group ? 'Rename group' : 'New group'} {onclose}>
  <div class="field">
    <label>Name</label>
    <input class="text" bind:this={input} bind:value={name} placeholder="Review"
           onkeydown={(e) => { if (e.key === 'Enter') save(); }} />
  </div>
  {#snippet footer()}
    <button class="btn ghost" onclick={onclose}>Cancel</button>
    <button class="btn primary" onclick={save} disabled={!name.trim()}>{group ? 'Save' : 'Create'}</button>
  {/snippet}
</Modal>
