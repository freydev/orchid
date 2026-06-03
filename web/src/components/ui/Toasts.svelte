<script lang="ts">
  import { toasts } from '../../lib/toast.svelte';
  import Icon from './Icon.svelte';
</script>

<div class="toasts">
  {#each toasts as t (t.id)}
    <div class="toast" class:err={t.kind === 'error'} class:ok={t.kind === 'ok'}>
      <Icon name={t.kind === 'error' ? 'warn' : 'check'} size={14} />
      <span>{t.text}</span>
      {#if t.mono}<span class="mono">{t.mono}</span>{/if}
    </div>
  {/each}
</div>

<style>
  .toasts { position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%); display: flex; flex-direction: column; gap: 8px; z-index: 60; align-items: center; }
  /* toasts above the status bar */
  .toasts { bottom: calc(var(--statusbar-h) + 14px); }
  .toast { max-width: 460px; background: var(--surface-3); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 9px 14px; font-size: 12.5px; display: flex; align-items: center; gap: 9px; box-shadow: 0 10px 30px rgba(0,0,0,0.4); animation: pop .16s; }
  .toast > :global(svg) { flex-shrink: 0; color: var(--text-dim); }
  .toast .mono { color: var(--text-dim); font-size: 11.5px; font-family: var(--mono); }
  .toast.err { border-color: var(--danger); }
  .toast.err > :global(svg) { color: var(--danger); }
  .toast.ok > :global(svg) { color: var(--ok); }
  .toast.out { animation: toastOut .2s forwards; }
  @keyframes toastOut { to { opacity: 0; transform: translateY(6px); } }
  @keyframes pop { from { transform: translateY(8px) scale(.98); opacity: 0; } }
</style>
