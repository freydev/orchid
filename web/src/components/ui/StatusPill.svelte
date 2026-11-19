<script lang="ts">
  import type { Status } from '../../lib/types';
  import { statusMeta } from '../../lib/util';

  let { status, id }: { status: Status; id?: string } = $props();

  const meta = $derived(statusMeta(status));
</script>

<span class="status-pill {meta.cls}" data-status={id}>
  <span class="dot {meta.cls}" class:pulse={meta.pulse}></span><span class="label">{meta.label}</span></span>

<style>
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.idle { background: var(--idle); }
  .dot.stopped { background: #3a3a42; }
  .dot.wait { background: var(--wait); }
  /* composited-only (transform/opacity): box-shadow pulse repainted every frame */
  .dot.pulse { position: relative; animation: pulse 1.3s ease-in-out infinite; }
  .dot.ok.pulse { color: var(--ok); }
  .dot.wait.pulse { color: var(--wait); }
  .dot.pulse::after {
    content: ''; position: absolute; inset: 0; border-radius: 50%;
    background: currentColor; animation: pulse-ring 1.3s ease-out infinite;
  }
  @keyframes pulse { 50% { transform: scale(1.35); } }
  @keyframes pulse-ring {
    0%   { transform: scale(1); opacity: .65; }
    100% { transform: scale(3); opacity: 0; }
  }
  .status-pill {
    display: inline-flex; align-items: center; gap: 6px; white-space: nowrap;
    font-size: 11.5px; font-weight: 500; color: var(--text-dim);
  }
  /* running: text breathes via opacity (composited). The old background-position
     shimmer on background-clip:text repainted every frame -> constant CPU. */
  .status-pill.ok { color: var(--ok); }
  .status-pill.ok .label { animation: runbreathe 1.8s ease-in-out infinite; }
  .status-pill.ok .dot { color: var(--ok); }
  @keyframes runbreathe { 50% { opacity: .55; } }
  .status-pill.wait { color: var(--wait); }
  .status-pill.stopped { color: var(--text-faint); }
</style>
