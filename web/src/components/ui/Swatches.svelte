<script lang="ts">
  import { DOT_COLORS } from '../../lib/theme';
  import Icon from './Icon.svelte';

  let {
    value = null, allowNone = false, noneLabel = 'Default', onpick,
  }: {
    value?: string | null; allowNone?: boolean; noneLabel?: string; onpick: (id: string) => void;
  } = $props();
</script>

<div class="swatches">
  {#if allowNone}
    <button type="button" class="swatch none" class:sel={!value} title={noneLabel} onclick={() => onpick('')}><Icon name="x" size={12} /></button>
  {/if}
  {#each DOT_COLORS as c (c.id)}
    <button type="button" class="swatch" class:sel={value === c.id} style="--sw:{c.hex}" title={c.id} onclick={() => onpick(c.id)}></button>
  {/each}
</div>

<style>
  .swatches { display: flex; flex-wrap: wrap; gap: 8px; }
  .swatch { width: 26px; height: 26px; border-radius: var(--radius-sm); background: var(--sw); border: 2px solid transparent; box-shadow: inset 0 0 0 1px rgba(255,255,255,0.08); cursor: pointer; transition: transform .08s; position: relative; }
  .swatch:hover { transform: scale(1.08); }
  .swatch.sel { border-color: var(--text); }
  .swatch.sel::after { content: ""; position: absolute; inset: 0; margin: auto; width: 7px; height: 7px; border-radius: 50%; background: rgba(0,0,0,0.55); }
  .swatch.none { background: var(--surface-3); display: grid; place-items: center; color: var(--text-faint); box-shadow: inset 0 0 0 1px var(--border-2); }
  .swatch.none.sel { border-color: var(--text); color: var(--text); }
  .swatch.none::after { display: none; }
</style>
