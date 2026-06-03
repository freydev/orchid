<script lang="ts">
  import { confirmState, resolveConfirm } from '../lib/modals.svelte';

  function onScrimDown(e: MouseEvent) {
    if (e.target === e.currentTarget) resolveConfirm(false);
  }
  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') resolveConfirm(false);
    else if (e.key === 'Enter') resolveConfirm(true);
  }

  // autofocus the confirm button on mount
  let yesBtn = $state<HTMLButtonElement | null>(null);
  $effect(() => { yesBtn?.focus(); });
</script>

<svelte:window on:keydown={onKey} />

<div class="scrim" onmousedown={onScrimDown}>
  <div class="modal">
    <div class="m-head"><h2>{confirmState.title}</h2></div>
    <div class="m-body"><p class="confirm-body">{@html confirmState.body}</p></div>
    <div class="m-foot">
      <button class="btn ghost" onclick={() => resolveConfirm(false)}>Cancel</button>
      <button
        bind:this={yesBtn}
        class="btn {confirmState.danger ? 'danger-solid' : 'primary'}"
        onclick={() => resolveConfirm(true)}
      >{confirmState.confirmLabel}</button>
    </div>
  </div>
</div>
