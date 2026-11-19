<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  let { data }: { data: any } = $props();
</script>

<div class="leaf"><Handle type="target" position={Position.Left} />{data.text}</div>

<style>
  .leaf { background: transparent; color: var(--text-faint); font-size: 11px; font-family: var(--mono); padding: 6px 8px; box-sizing: border-box; width: 180px; height: 32px; display: flex; align-items: center; }
</style>
