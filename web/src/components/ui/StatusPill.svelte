<script lang="ts">
  import type { Status } from '../../lib/types';
  import { statusMeta } from '../../lib/util';

  let { status, id }: { status: Status; id?: string } = $props();

  const meta = $derived(statusMeta(status));
</script>

<span class="status-pill {meta.cls}" data-status={id}>
  <span class="dot {meta.cls}" class:pulse={meta.pulse}></span>{meta.label}</span>

<style>
  .dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .dot.ok { background: var(--ok); }
  .dot.idle { background: var(--idle); }
  .dot.stopped { background: #3a3a42; }
  .dot.wait { background: var(--wait); }
  .dot.pulse { animation: pulse 1.3s ease-in-out infinite; }
  .dot.ok.pulse { color: var(--ok); }
  .dot.wait.pulse { color: var(--wait); }
  @keyframes pulse {
    0%   { box-shadow: 0 0 0 0 color-mix(in srgb, currentColor 65%, transparent); transform: scale(1); }
    50%  { box-shadow: 0 0 0 5px color-mix(in srgb, currentColor 10%, transparent); transform: scale(1.35); }
    100% { box-shadow: 0 0 0 8px transparent; transform: scale(1); }
  }
  .status-pill {
    display: inline-flex; align-items: center; gap: 6px; white-space: nowrap;
    font-size: 11.5px; font-weight: 500; color: var(--text-dim);
  }
  /* running: a bright highlight sweeps across the word (dot keeps its own color) */
  .status-pill.ok {
    color: transparent;
    background: linear-gradient(100deg, var(--ok) 0%, var(--ok) 42%, #eafff4 50%, var(--ok) 58%, var(--ok) 100%);
    background-size: 220% 100%;
    -webkit-background-clip: text;
    background-clip: text;
    animation: runshimmer 1.8s linear infinite;
  }
  .status-pill.ok .dot { color: var(--ok); }
  @keyframes runshimmer { from { background-position: 220% 0; } to { background-position: -120% 0; } }
  .status-pill.wait { color: var(--wait); }
  .status-pill.stopped { color: var(--text-faint); }
</style>
