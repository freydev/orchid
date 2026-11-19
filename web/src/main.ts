import './styles/tokens.css';
import './styles/base.css';
import { mount } from 'svelte';
import App from './App.svelte';
import { app } from './lib/state.svelte';
import { api } from './lib/api';
import { applyAccent, applyMonokai, applyRailWidth, DEFAULT_ACCENT } from './lib/theme';
import { start } from './lib/router';
import { pollStatus } from './lib/poll';

// A desktop-notification deep link arrives as `?s=<projectId>~<sessionId>`
// (a query, not a hash, so it survives a PWA "navigate-existing" launch which
// drops the fragment). Convert it to the hash route the router understands.
function consumeDeepLink() {
  let s = new URLSearchParams(location.search).get('s');
  if (!s) return;
  // PWA protocol_handlers deliver the value as the full "web+orchid://<...>"
  // URL; a plain browser open uses the bare value. Accept both.
  s = s.replace(/^web\+orchid:\/\//i, '');
  const i = s.indexOf('~');
  if (i < 0) return;
  const pid = s.slice(0, i), sid = s.slice(i + 1);
  if (!pid || !sid) return;
  const hash = '#/p/' + encodeURIComponent(pid) + '/terminal/' + encodeURIComponent(sid);
  history.replaceState(null, '', location.pathname + hash); // drop ?s, set route
}

async function boot() {
  consumeDeepLink();
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
