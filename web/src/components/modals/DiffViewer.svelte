<script lang="ts">
  import hljs from 'highlight.js';
  import 'highlight.js/styles/atom-one-dark.css';
  import Icon from '../ui/Icon.svelte';
  import { app } from '../../lib/state.svelte';
  import { api } from '../../lib/api';
  import type { Worktree, Diff, DiffFile, DiffLine } from '../../lib/types';

  let { wt, onclose }: { wt: Worktree; onclose: () => void } = $props();

  const DSTAT: Record<string, string> = { modified: 'M', added: 'A', deleted: 'D', renamed: 'R' };
  // map a file extension to a highlight.js language (only when registered)
  const DIFF_LANG: Record<string, string> = {
    js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript', ts: 'typescript', tsx: 'typescript',
    go: 'go', py: 'python', rb: 'ruby', rs: 'rust', java: 'java', kt: 'kotlin', c: 'c', h: 'c',
    cpp: 'cpp', cc: 'cpp', hpp: 'cpp', cs: 'csharp', php: 'php', swift: 'swift', dart: 'dart', lua: 'lua',
    sh: 'bash', bash: 'bash', zsh: 'bash', fish: 'bash', json: 'json', yml: 'yaml', yaml: 'yaml',
    toml: 'ini', ini: 'ini', md: 'markdown', html: 'xml', xml: 'xml', css: 'css', scss: 'scss', sql: 'sql', vim: 'vim',
  };
  function diffLang(path: string): string | null {
    const ext = (path.split('.').pop() || '').toLowerCase();
    const lang = DIFF_LANG[ext];
    return (lang && hljs.getLanguage(lang)) ? lang : null;
  }
  const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  // highlighted (or escaped) HTML for one diff line (trusted output → {@html})
  function dcodeHtml(text: string, lang: string | null): string {
    if (!text) return ' ';
    if (lang) { try { return hljs.highlight(text, { language: lang, ignoreIllegal: true }).value; } catch (_) {} }
    return esc(text);
  }

  let loading = $state(true);
  let errText = $state('');
  let diff = $state<Diff | null>(null);
  // per-file collapse state, keyed by file path
  let collapsed = $state<Record<string, boolean>>({});

  const totals = $derived.by(() => (diff?.files || []).reduce(
    (a, f) => ({ add: a.add + f.add, del: a.del + f.del }), { add: 0, del: 0 }));
  const modeLbl = $derived(diff?.mode === 'uncommitted' ? 'uncommitted' : 'last commit');
  // 'last_commit' → 'last-commit' to match the .dh-mode.last-commit selector
  const modeCls = $derived((diff?.mode || '').replace(/_/g, '-'));

  function toggleFile(path: string) { collapsed[path] = !collapsed[path]; }

  function gutter(t: DiffLine['t']): string { return t === 'add' ? '+' : t === 'del' ? '−' : ' '; }

  (async () => {
    try {
      const d = await api.diff(app.cur.projectId!, wt.path);
      // collapse files by default when there are many, to keep the list scannable
      const collapseAll = (d.files || []).length > 4;
      const c: Record<string, boolean> = {};
      for (const f of d.files || []) c[f.path] = collapseAll;
      collapsed = c;
      diff = d;
    } catch (e) {
      errText = (e as Error).message;
    } finally {
      loading = false;
    }
  })();

  function onKey(e: KeyboardEvent) { if (e.key === 'Escape') onclose(); }
</script>

<svelte:window onkeydown={onKey} />

<div
  class="scrim"
  role="presentation"
  onmousedown={(e) => { if (e.target === e.currentTarget) onclose(); }}
>
  <div class="diff-modal" role="dialog" aria-modal="true" onmousedown={(e) => e.stopPropagation()}>
    <div class="diff-head">
      <div class="dh-id">
        <div class="dh-title">
          <Icon name="diff" size={16} /> Diff
          {#if diff}<span class="dh-mode {modeCls}">{modeLbl}</span>{/if}
        </div>
        <div class="dh-wt mono"><Icon name="branch" size={12} /> {wt.name}</div>
      </div>
      {#if diff}
        <div class="dh-meta">
          <span class="dh-ref mono">{diff.ref}</span>
          <span class="dh-sub">{diff.sub}</span>
        </div>
        <span class="spacer"></span>
        <div class="dh-totals">
          <span class="dadd">+{totals.add}</span>
          <span class="ddel">−{totals.del}</span>
        </div>
      {:else}
        <span class="spacer"></span>
      {/if}
      <button class="btn sm icon ghost" title="Close" onclick={onclose}><Icon name="x" size={18} /></button>
    </div>

    <div class="diff-body">
      {#if loading}
        <div class="diff-empty">Loading…</div>
      {:else if errText}
        <div class="diff-empty">Failed to load diff: {errText}</div>
      {:else if diff && (diff.files || []).length}
        {#each diff.files as f (f.path)}
          {@const lang = diffLang(f.path)}
          <div class="dfile">
            <button class="dfile-head" type="button" onclick={() => toggleFile(f.path)}>
              <span class="dfile-chev" class:open={!collapsed[f.path]}><Icon name="chevR" size={13} /></span>
              <span class="dstat {f.status}">{DSTAT[f.status] || 'M'}</span>
              <span class="dpath mono">{f.path}</span>
              <span class="spacer"></span>
              {#if f.add > 0}<span class="dadd">+{f.add}</span>{/if}
              {#if f.del > 0}<span class="ddel">−{f.del}</span>{/if}
            </button>
            <div class="dhunks" style:display={collapsed[f.path] ? 'none' : ''}>
              {#each f.hunks || [] as h}
                <div class="dhunk">
                  <div class="dhunk-head mono">{h.header}</div>
                  {#each h.lines as ln}
                    <div class="dline {ln.t}">
                      <span class="dgutter">{gutter(ln.t)}</span>
                      <span class="dcode mono">{@html dcodeHtml(ln.text, lang)}</span>
                    </div>
                  {/each}
                </div>
              {/each}
            </div>
          </div>
        {/each}
      {:else}
        <div class="diff-empty">No changes.</div>
      {/if}
    </div>
  </div>
</div>

<style>
  .diff-modal { width: 860px; max-width: calc(100vw - 40px); height: 80vh; max-height: 780px; background: var(--surface); border: 1px solid var(--border-2); border-radius: var(--radius-sm); box-shadow: 0 24px 60px rgba(0,0,0,0.6); display: flex; flex-direction: column; animation: pop .16s; }
  .diff-head { display: flex; align-items: center; gap: 16px; padding: 13px 16px; border-bottom: 1px solid var(--border); flex-shrink: 0; }
  .dh-id { display: flex; flex-direction: column; gap: 3px; }
  .dh-title { display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 600; letter-spacing: -0.01em; }
  .dh-title > :global(svg) { color: var(--accent); }
  .dh-mode { font-family: var(--mono); font-size: 9.5px; padding: 2px 6px; }
  .dh-mode.uncommitted { color: var(--wait); background: var(--wait-bg); }
  .dh-mode.last-commit { color: var(--text-dim); background: var(--surface-3); }
  .dh-wt { display: flex; align-items: center; gap: 5px; font-size: 11px; color: var(--text-faint); }
  .dh-wt > :global(svg) { flex-shrink: 0; }
  .dh-meta { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
  .dh-ref { font-size: 12px; color: var(--text); }
  .dh-sub { font-size: 11px; color: var(--text-faint); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 320px; }
  .diff-head .spacer { flex: 1; }
  .dh-totals { display: flex; gap: 10px; font-family: var(--mono); font-size: 12px; font-weight: 600; }
  .dadd { color: var(--ok); }
  .ddel { color: var(--danger); }
  .diff-body { flex: 1; min-height: 0; overflow-y: auto; padding: 12px; display: flex; flex-direction: column; gap: 10px; background: var(--bg); }
  .diff-empty { padding: 60px; text-align: center; color: var(--text-faint); }
  .dfile { border: 1px solid var(--border); background: var(--surface); }
  .dfile-head { display: flex; align-items: center; gap: 9px; padding: 9px 12px; cursor: pointer; user-select: none; width: 100%; text-align: left; }
  .dfile-head:hover { background: var(--surface-2); }
  .dfile-chev { display: inline-flex; color: var(--text-faint); transition: transform .12s; }
  .dfile-chev.open { transform: rotate(90deg); }
  .dstat { width: 17px; height: 17px; display: grid; place-items: center; font-family: var(--mono); font-size: 10px; font-weight: 700; flex-shrink: 0; }
  .dstat.modified { color: var(--wait); background: var(--wait-bg); }
  .dstat.added { color: var(--ok); background: var(--ok-bg); }
  .dstat.deleted { color: var(--danger); background: rgba(229,115,115,0.13); }
  .dstat.renamed { color: var(--accent); background: var(--accent-bg); }
  .dpath { font-size: 12.5px; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .dfile-head .spacer { flex: 1; }
  .dfile-head .dadd, .dfile-head .ddel { font-family: var(--mono); font-size: 11.5px; }
  .dhunks { border-top: 1px solid var(--border); }
  .dhunk-head { font-size: 11px; color: var(--accent); background: var(--accent-bg); padding: 3px 12px; }
  .dline { display: flex; font-size: 12px; line-height: 1.55; }
  .dgutter { width: 24px; flex-shrink: 0; text-align: center; font-family: var(--mono); color: var(--text-faint); user-select: none; }
  .dcode { white-space: pre-wrap; word-break: break-word; padding-right: 12px; flex: 1; }
  .dline.ctx { color: var(--text-dim); }
  .dline.add { background: rgba(78,232,138,0.10); }
  .dline.add .dgutter { color: var(--ok); }
  .dline.del { background: rgba(240,114,106,0.10); }
  .dline.del .dgutter { color: var(--danger); }
  /* rail diff icon button */
  .rail-icon { color: var(--text-faint); display: inline-flex; padding: 0 4px; flex-shrink: 0; }
  .rail-icon:hover { color: var(--accent); }
</style>
