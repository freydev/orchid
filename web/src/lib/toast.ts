// runes toast store + helper — ported from web/app.js `toast()`
let seq = 0;
export const toasts = $state([] as { id: number; text: string; kind: string; mono?: string }[]);
export function toast(text: string, kind: 'info' | 'ok' | 'error' = 'info', mono?: string) {
  const id = ++seq;
  toasts.push({ id, text, kind, mono });
  setTimeout(() => { const i = toasts.findIndex((t) => t.id === id); if (i >= 0) toasts.splice(i, 1); }, kind === 'error' ? 4200 : 2600);
}
