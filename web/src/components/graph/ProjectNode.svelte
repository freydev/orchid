<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { colorHex } from '../../lib/theme';
  import Icon from '../ui/Icon.svelte';
  let { data }: { data: any } = $props();
  const p = $derived(data.project);
  const pcol = $derived(colorHex(p.color) || 'var(--accent)');
  const expanded = $derived(data.cb.expandedProjects().has(p.id));
  const list = $derived(p.session_list || []);
  const run = $derived(list.filter((s: any) => s.status === 'running').length);
  const wait = $derived(list.filter((s: any) => s.status === 'requires_action').length);
  const idle = $derived(list.filter((s: any) => s.status === 'idle' || s.status === 'stopped').length);
  const branch = $derived((p.branch || '').trim());
  const path = $derived((p.path || '').trim());

  let editing = $state(false);
  let draft = $state('');
  let inputEl = $state<HTMLInputElement | null>(null);
  function startEdit() { draft = p.name; editing = true; }
  function commit() { if (!editing) return; editing = false; data.cb.renameProject(p, draft); }
  function cancel() { editing = false; }
  $effect(() => { if (editing) inputEl?.focus(); });
</script>

<div class="gnode project" style="--pcol:{pcol}">
  <div class="row">
    <button class="pdot" title="Open project" aria-label="Open project" onclick={() => data.cb.openProject(p.id)}></button>
    <button class="caret" class:open={expanded} title="Expand" onclick={() => data.cb.toggleProject(p.id)}><Icon name="chevR" size={13} /></button>
    {#if editing}
      <input class="rename nodrag" bind:this={inputEl} bind:value={draft}
        onkeydown={(e) => { e.stopPropagation(); if (e.key === 'Enter') commit(); else if (e.key === 'Escape') cancel(); }}
        onblur={cancel} />
    {:else}
      <button class="title" title="Click to rename" onclick={startEdit}>{p.name}</button>
    {/if}
  </div>

  {#if path}
    <div class="brow"><span class="ppath" title={path}><Icon name="folder" size={11} /><span class="ptxt">{path}</span></span></div>
  {/if}

  {#if branch}
    <div class="brow"><span class="pbranch" title="current branch"><Icon name="branch" size={11} />{branch}</span></div>
  {/if}

  <div class="foot">
    <div class="meta">
      <span class="c ok">{run} run</span><span class="c wait">{wait} wait</span><span class="c idle">{idle} idle</span>
    </div>
    <div class="spacer"></div>
    <button class="ic" title="New worktree" onclick={() => data.cb.newWorktree(p)}><Icon name="branch" size={14} /></button>
    <button class="ic" title="Project settings" onclick={() => data.cb.openProjectSettings(p)}><Icon name="gear" size={15} /></button>
    <button class="ic danger" title="Delete project" onclick={() => data.cb.deleteProject(p)}><Icon name="x" size={16} /></button>
  </div>
  <Handle type="source" position={Position.Right} />
</div>

<style>
  /* PROJECT = heavy "header" card: elevated surface, project-color bar + tint, bold */
  .gnode { background: linear-gradient(90deg, color-mix(in srgb, var(--pcol) 14%, transparent), transparent 220px), var(--surface-2); border: 1px solid var(--border-2); border-left: 4px solid var(--pcol); border-radius: var(--radius); padding: 10px 13px; box-sizing: border-box; width: 290px; height: 132px; display: flex; flex-direction: column; gap: 6px; box-shadow: 0 1px 0 rgba(0,0,0,0.25); }
  .row { display: flex; align-items: center; gap: 8px; }
  .pdot { width: 12px; height: 12px; border-radius: 50%; background: var(--pcol); flex-shrink: 0; box-shadow: inset 0 0 0 1px rgba(255,255,255,0.15); transition: transform .1s; }
  .pdot:hover { transform: scale(1.15); }
  .caret { color: var(--text-faint); transition: transform .15s; display: inline-flex; }
  .caret.open { transform: rotate(90deg); }
  .title { flex: 1; font-weight: 700; font-size: 14.5px; letter-spacing: -0.01em; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .title:hover { color: var(--accent); }
  .rename { flex: 1; min-width: 0; font-size: 14px; font-weight: 600; background: var(--surface); border: 1px solid var(--accent); border-radius: var(--radius-sm); color: var(--text); padding: 2px 7px; }
  .brow { display: flex; min-width: 0; }
  .pbranch { display: inline-flex; align-items: center; gap: 4px; font-family: var(--mono); font-size: 10.5px; color: var(--text-dim); background: var(--surface); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 1px 7px 1px 5px; max-width: 100%; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .pbranch > :global(svg) { color: var(--text-faint); flex-shrink: 0; }
  .ppath { display: inline-flex; align-items: center; gap: 4px; font-family: var(--mono); font-size: 10.5px; color: var(--text-dim); min-width: 0; max-width: 100%; }
  .ppath > :global(svg) { color: var(--pcol); flex-shrink: 0; }
  /* truncate the path from the LEFT so the project folder (tail) stays visible */
  .ptxt { min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; direction: rtl; unicode-bidi: plaintext; }
  .foot { display: flex; align-items: center; gap: 8px; margin-top: auto; }
  .meta { display: flex; gap: 8px; font-size: 10.5px; }
  .c.ok { color: var(--ok); } .c.wait { color: var(--wait); } .c.idle { color: var(--text-dim); }
  .spacer { flex: 1; }
  .ic { color: var(--text-faint); display: inline-flex; } .ic:hover { color: var(--text); } .ic.danger:hover { color: var(--danger); }
</style>
