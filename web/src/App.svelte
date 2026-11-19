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
  import GlobalSettings from './components/modals/GlobalSettings.svelte';
  import ProjectSettings from './components/modals/ProjectSettings.svelte';
  import SessionSettings from './components/modals/SessionSettings.svelte';
  import DiffViewer from './components/modals/DiffViewer.svelte';
  import GroupName from './components/modals/GroupName.svelte';
  import { modal, confirmState, closeModal, openModal } from './lib/modals.svelte';
  import { VERSION } from './lib/version';
  import GraphPage from './components/graph/GraphPage.svelte';
  import { setHash } from './lib/router';
</script>

<div class="app">
  <div class="topbar">
    <div class="brand">
      <div class="mark">
        <img src="/orchid-logo.svg" alt="Orchid" />
      </div>
      Orchid<span class="ver">tmux multiplexer · v{VERSION}</span>
    </div>
    <div class="spacer"></div>
    <div class="view-seg">
      <button class:sel={!app.graph} title="List view" onclick={() => { app.graph = false; setHash(); }}><Icon name="list" size={15} /></button>
      <button class:sel={app.graph} title="Graph view" onclick={() => { app.graph = true; setHash(); }}><Icon name="graph" size={15} /></button>
    </div>
    <button class="topbar-gear" title="Settings" onclick={() => openModal('globalSettings')}><Icon name="gear" size={18} /></button>
  </div>
  <div class="main">
    {#if app.graph}<GraphPage />{:else if app.cur.projectId}<ProjectDetail />{:else}<ProjectsPage />{/if}
  </div>
  <Statusbar />
  <Toasts />

  {#if confirmState.open}<Confirm />{/if}

  <!-- modal host — extend this chain as modal components land in later tasks -->
  {#if modal.kind === 'newProject'}<NewProject onclose={closeModal} />
  {:else if modal.kind === 'newWorktree'}<NewWorktree onclose={closeModal} />
  {:else if modal.kind === 'newSession'}<NewSession {...modal.props} onclose={closeModal} />
  {:else if modal.kind === 'globalSettings'}<GlobalSettings onclose={closeModal} />
  {:else if modal.kind === 'projectSettings'}<ProjectSettings {...modal.props} onclose={closeModal} />
  {:else if modal.kind === 'sessionSettings'}<SessionSettings {...modal.props} onclose={closeModal} />
  {:else if modal.kind === 'diff'}<DiffViewer {...modal.props} onclose={closeModal} />
  {:else if modal.kind === 'groupName'}<GroupName {...modal.props} onclose={closeModal} />
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
    height: 24px;
    background: none;
    display: grid; place-items: center;
  }
  .brand .mark img { height: 24px; width: auto; display: block; }
  .brand .ver { font-family: var(--mono); font-size: 11px; color: var(--text-faint); font-weight: 400; padding-top: 1px; }
  .topbar .spacer { flex: 1; }

  .main { flex: 1; min-height: 0; overflow: auto; scrollbar-gutter: stable; }

  /* topbar settings gear */
  .topbar-gear { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: var(--radius-sm); color: var(--text-dim); }
  .topbar-gear:hover { color: var(--accent); background: var(--surface-2); border-color: var(--border-2); }

  .view-seg { display: flex; background: var(--surface-2); border: 1px solid var(--border-2); border-radius: var(--radius-sm); padding: 2px; gap: 2px; }
  .view-seg button { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 26px; border-radius: var(--radius-sm); color: var(--text-dim); }
  .view-seg button.sel { background: var(--accent-bg); color: var(--accent); }
  .view-seg button:hover:not(.sel) { color: var(--text); }

  @media (max-width: 640px) {
    .topbar { height: 58px; padding: 0 14px; gap: 10px; }
    .brand { font-size: 15px; gap: 8px; }
    .brand .mark { height: 40px; }
    .brand .mark img { height: 40px; }
    .brand .ver { display: none; }
    .topbar-gear { width: 40px; height: 40px; }
  }
</style>
