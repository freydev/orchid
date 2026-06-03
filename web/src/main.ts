import './styles/tokens.css';
import './styles/base.css';
import { mount } from 'svelte';
import App from './App.svelte';
import { app } from './lib/state.svelte';
import { api } from './lib/api';
import { applyAccent, applyMonokai, applyRailWidth, DEFAULT_ACCENT } from './lib/theme';
import { start } from './lib/router';
import { pollStatus } from './lib/poll';

async function boot() {
  try { app.settings = await api.settings(); } catch {}
  try { app.shellName = (await api.shell()).name; } catch {}
  const accent = app.settings.accent || DEFAULT_ACCENT;
  if (accent === 'monokai') applyMonokai(); else applyAccent(accent);
  applyRailWidth(app.settings.rail_width || 230);
  await start();
  setInterval(pollStatus, 3000);
}
const target = document.getElementById('root')!;
boot().finally(() => mount(App, { target }));
export {};
