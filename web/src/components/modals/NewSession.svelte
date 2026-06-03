<script lang="ts">
  import Modal from '../Modal.svelte';
  import Swatches from '../ui/Swatches.svelte';
  import Badge from '../ui/Badge.svelte';
  import Icon from '../ui/Icon.svelte';
  import { app, refresh } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import { toast } from '../../lib/toast.svelte';
  import { shellName, relTime } from '../../lib/util';
  import { setHash } from '../../lib/router';
  import type { ClaudeSession } from '../../lib/types';

  let { wt, onclose }: { wt?: string; onclose: () => void } = $props();

  const globalShell = () => app.shellName || (app.settings.shells && app.settings.shells[0]) || 'bash';
  function shellOptions(value: string): string[] {
    const names = (app.settings.shells && app.settings.shells.length) ? app.settings.shells : ['zsh', 'bash', 'fish', 'sh'];
    const v = shellName(value);
    const list = names.slice();
    if (v && !list.includes(v)) list.unshift(v);
    return list;
  }

  const proj = app.projects.find((p) => p.id === app.cur.projectId) || ({} as any);
  const presets = proj.commands || [];
  const defShell = shellName(proj.shell) || globalShell();

  const types = [
    { id: 'claude', l: 'Claude', d: 'AI agent' },
    { id: 'codex', l: 'Codex', d: 'AI agent' },
    { id: 'shell', l: 'Shell', d: app.shellName + ' shell' },
  ];

  const wtOpts = app.worktrees.filter((w) => !app.removingWt.has(w.path));

  let name = $state('');
  let namePlaceholder = $state('router + sessions API');
  let worktree = $state(wtOpts.some((w) => w.path === wt) ? wt! : (wtOpts[0]?.path ?? ''));
  let type = $state<'claude' | 'codex' | 'shell'>('claude');
  let color = $state('');
  let shell = $state(defShell);
  let cmdId = $state('');

  const shellList = shellOptions(shell);

  // resumable claude conversations for the selected worktree
  let resumeSessions = $state<ClaudeSession[]>([]);
  let resumeId = $state('');
  let resumeLoadedCwd: string | null = null;

  async function loadResume(cwd: string) {
    if (cwd === resumeLoadedCwd) return;
    resumeLoadedCwd = cwd;
    resumeId = '';
    try {
      resumeSessions = (await api.claudeSessions(cwd)) || [];
    } catch {
      resumeSessions = [];
    }
  }

  // load resume list when claude is selected and on worktree change
  $effect(() => {
    if (type === 'claude' && worktree) loadResume(worktree);
  });

  function pickType(t: 'claude' | 'codex' | 'shell') {
    type = t;
  }

  function pickResume() {
    const s = resumeSessions.find((x) => x.id === resumeId);
    if (resumeId && s && s.title && !name.trim()) name = s.title;
  }

  function pickPreset(id: string) {
    cmdId = id;
    const chosen = presets.find((p: any) => p.id === cmdId);
    namePlaceholder = chosen ? chosen.label : 'router + sessions API';
  }

  async function create() {
    const chosen = type === 'shell' ? presets.find((p: any) => p.id === cmdId) : null;
    const finalName = name.trim() || (chosen ? chosen.label : '');
    const body: any = { project: app.cur.projectId, worktree, agent: type, name: finalName, color };
    if (type === 'shell') { body.shell = shell; if (chosen) body.command = chosen.cmd; }
    if (type === 'claude' && resumeId) body.resume_id = resumeId;
    try {
      const s = await api.createSession(body);
      onclose();
      await refresh();
      // open the Terminal for the new session
      app.cur.view = 'terminal';
      app.cur.activeId = s.id;
      setHash();
    } catch (err) {
      toast((err as Error).message, 'error');
    }
  }
</script>

<Modal title="New session" {onclose}>
  <div class="field">
    <label>Name (optional)</label>
    <input class="text" bind:value={name} placeholder={namePlaceholder} />
  </div>
  <div class="field">
    <label>Worktree</label>
    <select class="text mono" bind:value={worktree}>
      {#each wtOpts as w}<option value={w.path}>{w.name}</option>{/each}
    </select>
  </div>
  <div class="field">
    <label>Type</label>
    <div class="choice-grid">
      {#each types as t}
        <button type="button" class="choice" class:sel={type === t.id} onclick={() => pickType(t.id as any)}>
          <Badge agent={t.id} lg={true} />
          <span class="lbl">{t.l}</span>
          <span class="desc">{t.d}</span>
        </button>
      {/each}
    </div>
  </div>
  <div class="field">
    <label>Color</label>
    <Swatches value={color} allowNone={true} noneLabel="Agent default" onpick={(c) => (color = c)} />
  </div>

  {#if type === 'claude'}
    <div class="field claude-field">
      <label>Resume Claude session<span class="fhint">import an existing conversation</span></label>
      <select class="text" bind:value={resumeId} onchange={pickResume}>
        <option value="">Start new conversation</option>
        {#each resumeSessions as s}
          <option value={s.id}>{(s.title || ('session ' + s.id.slice(0, 8))) + ' · ' + relTime(s.updated)}</option>
        {/each}
      </select>
    </div>
  {/if}

  {#if type === 'shell'}
    <div class="field shell-field">
      <label>Command preset<span class="fhint">opens a tmux session running it</span></label>
      <div class="cmd-presets">
        <button type="button" class="cmd-chip" class:sel={cmdId === ''} onclick={() => pickPreset('')}>
          <Icon name="term" size={14} /> Plain shell
        </button>
        {#each presets as p}
          <button type="button" class="cmd-chip" class:sel={cmdId === p.id} onclick={() => pickPreset(p.id)}>
            <span class="cc-label">{p.label}</span><span class="cc-cmd mono">{p.cmd}</span>
          </button>
        {/each}
      </div>
      {#if !presets.length}
        <div class="cmd-empty">No presets yet — add them in project settings.</div>
      {/if}
    </div>
    <div class="field shell-field">
      <label>Shell<span class="fhint">for this session</span></label>
      <select class="text" bind:value={shell}>
        {#each shellList as n}<option value={n}>{n}</option>{/each}
      </select>
    </div>
  {/if}

  {#snippet footer()}
    <button class="btn ghost" onclick={onclose}>Cancel</button>
    <button class="btn primary" onclick={create}>Start</button>
  {/snippet}
</Modal>
