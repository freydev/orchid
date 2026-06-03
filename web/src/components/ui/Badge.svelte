<script lang="ts">
  import { agentMeta } from '../../lib/util';
  import { colorHex } from '../../lib/theme';

  let { agent, color = null, lg = false }: { agent: string; color?: string | null; lg?: boolean } = $props();
</script>

{#if color}
  <span class="s-badge-wrap">
    <div class="agent-badge {agent}" class:lg>{agentMeta(agent).badge}</div>
    <span class="s-color" style="background:{colorHex(color)}"></span>
  </span>
{:else}
  <div class="agent-badge {agent}" class:lg>{agentMeta(agent).badge}</div>
{/if}

<style>
  .agent-badge {
    width: 26px; height: 26px; border-radius: var(--radius-sm); flex-shrink: 0;
    display: grid; place-items: center;
    font-family: var(--mono); font-size: 11px; font-weight: 600;
    border: 1px solid var(--border-2);
  }
  .agent-badge.codex { background: rgba(123,211,193,0.10); color: #7bd3c1; border-color: rgba(123,211,193,0.25); }
  .agent-badge.claude { background: rgba(217,139,90,0.12); color: #e0a06f; border-color: rgba(217,139,90,0.28); }
  .agent-badge.shell { background: var(--surface-3); color: var(--text-dim); }
  .agent-badge.lg { width: 32px; height: 32px; border-radius: var(--radius-sm); font-size: 13px; }
  .s-badge-wrap { position: relative; display: inline-flex; flex-shrink: 0; }
  .s-color { position: absolute; right: -3px; bottom: -3px; width: 10px; height: 10px; border-radius: 50%; border: 2px solid var(--surface); }
</style>
