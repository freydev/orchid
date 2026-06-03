<script lang="ts">
  import type { Snippet } from 'svelte';

  let {
    title,
    desc,
    onclose,
    children,
    footer,
  }: {
    title: string;
    desc?: string | Snippet;
    onclose: () => void;
    children: Snippet;
    footer?: Snippet;
  } = $props();

  function onScrimDown(e: MouseEvent) {
    if (e.target === e.currentTarget) onclose();
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }
</script>

<svelte:window on:keydown={onKey} />

<div class="scrim" onmousedown={onScrimDown}>
  <div class="modal">
    <div class="m-head">
      <h2>{title}</h2>
      {#if typeof desc === 'function'}
        {@render desc()}
      {:else if desc}
        <p>{@html desc}</p>
      {/if}
    </div>
    <div class="m-body">{@render children()}</div>
    {#if footer}
      <div class="m-foot">{@render footer()}</div>
    {/if}
  </div>
</div>
