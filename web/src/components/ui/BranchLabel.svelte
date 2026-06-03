<script lang="ts">
  import { parseBranch } from '../../lib/util';

  let { name }: { name: string } = $props();

  const parts = $derived(parseBranch(name));
</script>

{#if parts.tag}<span class="b-tag">{parts.tag}</span><span class="b-rest">{parts.rest}</span>{:else}<span class="b-rest">{parts.rest}</span>{/if}

<style>
  .b-tag { font-family: var(--mono); font-size: 9px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--accent); background: var(--accent-bg); padding: 0 4px; border-radius: var(--radius-sm); white-space: nowrap; flex-shrink: 0; line-height: 1.5; }
  .b-rest { font-family: var(--mono); }
</style>
