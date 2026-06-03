<script lang="ts">
  import { app } from './lib/state.svelte';
  import ProjectsPage from './components/ProjectsPage.svelte';
  import ProjectDetail from './components/ProjectDetail.svelte';
  import Statusbar from './components/Statusbar.svelte';
  import Toasts from './components/ui/Toasts.svelte';
  import Icon from './components/ui/Icon.svelte';
  import Confirm from './components/Confirm.svelte';
  import NewProject from './components/modals/NewProject.svelte';
  import NewWorktree from './components/modals/NewWorktree.svelte';
  import NewSession from './components/modals/NewSession.svelte';
  import { modal, confirmState, closeModal } from './lib/modals.svelte';
  // (global settings modal wiring comes in a later task)
</script>

<div class="app">
  <div class="topbar">
    <div class="brand">
      <div class="mark">
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><g opacity="0.5"><ellipse cx="12" cy="6.8" rx="3.8" ry="4.8"/><ellipse cx="12" cy="6.8" rx="3.8" ry="4.8" transform="rotate(120 12 12)"/><ellipse cx="12" cy="6.8" rx="3.8" ry="4.8" transform="rotate(240 12 12)"/></g><g><ellipse cx="12" cy="7.8" rx="1.7" ry="3.6" transform="rotate(60 12 12)"/><ellipse cx="12" cy="7.8" rx="1.7" ry="3.6" transform="rotate(180 12 12)"/><ellipse cx="12" cy="7.8" rx="1.7" ry="3.6" transform="rotate(300 12 12)"/></g><circle cx="12" cy="12" r="1.5" fill="var(--bg)"/></svg>
      </div>
      orchid<span class="ver">tmux · v0.5</span>
    </div>
    <div class="spacer"></div>
    <span class="kbd">codex</span>
    <span class="kbd">claude</span>
    <button class="topbar-gear" title="Settings"><Icon name="gear" size={18} /></button>
  </div>
  <div class="main">
    {#if app.cur.projectId}<ProjectDetail />{:else}<ProjectsPage />{/if}
  </div>
  <Statusbar />
  <Toasts />

  {#if confirmState.open}<Confirm />{/if}

  <!-- modal host — extend this chain as modal components land in later tasks -->
  {#if modal.kind === 'newProject'}<NewProject onclose={closeModal} />
  {:else if modal.kind === 'newWorktree'}<NewWorktree onclose={closeModal} />
  {:else if modal.kind === 'newSession'}<NewSession {...modal.props} onclose={closeModal} />
  {/if}
</div>

<style>
  .app { display: flex; flex-direction: column; height: 100%; }

  .topbar {
    display: flex; align-items: center; gap: 14px;
    height: 52px; padding: 0 18px;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
    flex-shrink: 0;
  }
  .brand { display: flex; align-items: center; gap: 9px; font-weight: 600; letter-spacing: -0.01em; }
  .brand .mark {
    width: 22px; height: 22px;
    background: none;
    display: grid; place-items: center; color: var(--accent);
  }
  .brand .mark svg { width: 22px; height: 22px; }
  .brand .ver { font-family: var(--mono); font-size: 11px; color: var(--text-faint); font-weight: 400; padding-top: 1px; }
  .topbar .spacer { flex: 1; }
  .kbd {
    font-family: var(--mono); font-size: 11px; color: var(--text-faint);
    border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 2px 6px;
  }

  .main { flex: 1; min-height: 0; overflow: auto; scrollbar-gutter: stable; }

  /* topbar settings gear */
  .topbar-gear { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: var(--radius-sm); color: var(--text-dim); }
  .topbar-gear:hover { color: var(--accent); background: var(--surface-2); border-color: var(--border-2); }
</style>
