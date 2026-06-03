<script lang="ts">
  import Modal from '../Modal.svelte';
  import { app, refresh } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import type { GitInfo } from '../../lib/types';

  let { onclose }: { onclose: () => void } = $props();

  let git = $state<GitInfo | null>(null);
  let branch = $state('');
  let from = $state('');
  let fetch = $state(true);
  let busy = $state(false);
  let ready = $state(false); // ok enabled once gitinfo resolved (or failed)
  let notRepo = $state(false);
  let step = $state(''); // non-empty => show spinner row
  let errText = $state('');

  // probe git info to show the real base + populate the "from" picker
  (async () => {
    try {
      const g = await api.gitInfo(app.cur.projectId!);
      git = g;
      if (!g.is_repo) { notRepo = true; ready = false; return; }
      const branches = g.branches || [];
      // preselect the project's configured base branch, else the repo default
      const proj = app.projects.find((p) => p.id === app.cur.projectId) || ({} as any);
      const want = (proj.base_branch || '').trim() || g.default_branch;
      if (branches.length) from = branches.includes(want) ? want : g.default_branch;
      ready = true;
    } catch {
      ready = true;
    }
  })();

  // existing branch -> checkout (base irrelevant); new -> branch off "from", optionally fetched
  const exists = $derived(!!git && (git.branches || []).includes(branch.trim()));
  const branches = $derived(git?.branches || []);
  const showFrom = $derived(!!git && git.is_repo && branches.length > 0);
  const showFetch = $derived(!!git && git.has_origin);

  const desc = $derived.by(() => {
    if (notRepo) return 'This project is not a git repository — cannot create a worktree.';
    if (!git) return 'Creates a new branch off the fresh default branch.';
    if (exists) return 'Branch ' + branch.trim() + ' exists — it will be checked out into the worktree.';
    const base = from || git.default_branch;
    const willFetch = git.has_origin && fetch;
    return willFetch
      ? 'New branch off origin/' + base + ' (fetched fresh).'
      : 'New branch off local ' + base + '.';
  });

  const okDisabled = $derived(notRepo || !ready || busy);

  async function submit() {
    if (busy || okDisabled) return;
    const b = branch.trim();
    if (!b) return;
    const doFetch = !!(git && git.has_origin && !exists && fetch);
    errText = '';
    busy = true;
    step = doFetch ? 'Fetching origin…' : 'Creating worktree…';
    const t = doFetch ? setTimeout(() => { step = 'Creating worktree…'; }, 700) : null;
    try {
      await api.addWorktree(app.cur.projectId!, { branch: b, from, fetch });
      if (t) clearTimeout(t);
      step = 'Done';
      onclose();
      await refresh();
    } catch (e) {
      if (t) clearTimeout(t);
      step = '';
      errText = 'Failed: ' + (e as Error).message;
      busy = false;
    }
  }
</script>

{#snippet descSnippet()}
  <p style:color={notRepo ? 'var(--danger)' : null}>{desc}</p>
{/snippet}

<Modal title="New worktree" desc={descSnippet} {onclose}>
  <div class="field">
    <label>Branch</label>
    <input
      class="text mono"
      bind:value={branch}
      placeholder="feat/new-thing"
      disabled={notRepo || busy}
      onkeydown={(e) => { if (e.key === 'Enter') submit(); }}
    />
  </div>

  {#if showFrom}
    <div class="field" style:opacity={exists ? '.45' : ''}>
      <label>From branch<span class="fhint">base for the new branch</span></label>
      <select class="text mono" bind:value={from} disabled={exists || busy}>
        {#each branches as b}<option value={b}>{b}</option>{/each}
      </select>
    </div>
  {/if}

  {#if showFetch}
    <label class="checkrow" style:opacity={exists ? '.45' : ''}>
      <input type="checkbox" bind:checked={fetch} disabled={exists || busy} />
      <span>Fetch <code>origin/&lt;branch&gt;</code> first<span class="fhint">off — branch off the local copy</span></span>
    </label>
  {/if}

  {#if step}
    <div class="load-status"><span class="spinner"></span><span class="step">{step}</span></div>
  {/if}
  {#if errText}
    <div class="load-err">{errText}</div>
  {/if}

  {#snippet footer()}
    <button class="btn ghost" onclick={() => { if (!busy) onclose(); }} disabled={busy}>Cancel</button>
    <button class="btn primary" onclick={submit} disabled={okDisabled}>Create</button>
  {/snippet}
</Modal>
