/* terminal: Svelte action wrapping xterm + FitAddon + WebSocket.
   Ported from web/app.js `attachInto`. */

import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { MONOKAI, termThemes } from "../lib/theme";
import { openSessionSocket, sendResize, sendBytes } from "../lib/ws";

export interface TerminalParams {
  id: string;
  readonly?: boolean;
  noResize?: boolean;
}

export function terminal(node: HTMLElement, params: TerminalParams): { destroy(): void } {
  const { id } = params;
  const ro_ = !!params.readonly; // grid preview panes: read-only, no cursor
  const noResize = !!params.noResize;

  // read-only previews: hide the cursor and disable stdin
  const theme = ro_
    ? Object.assign({}, MONOKAI, { cursor: "rgba(0,0,0,0)", cursorAccent: MONOKAI.background })
    : MONOKAI;
  const term = new Terminal({
    cursorBlink: !ro_,
    disableStdin: ro_,
    fontSize: 13.5,
    fontFamily: '"Fira Mono", "JetBrains Mono", ui-monospace, Menlo, monospace',
    scrollback: 5000,
    theme,
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(node);

  const fitNow = () => {
    fit.fit();
    if (noResize) { try { term.resize(220, term.rows); } catch (_) { /* ignore */ } }
  };
  fitNow();

  const ws = openSessionSocket(id);

  // Grid panes are size-passive: never push a resize to tmux. With
  // window-size=latest, a tiny grid pane would otherwise shrink the shared
  // pane and break the full-screen/focus rendering of the same session.
  const send = () => { if (noResize) return; sendResize(ws, term.cols, term.rows); };
  ws.onopen = () => send();
  ws.onmessage = (e) => { if (e.data instanceof ArrayBuffer) term.write(new Uint8Array(e.data)); };
  ws.onclose = () => {};

  // Shift+Enter inserts a newline instead of submitting. We deliver the newline
  // as a one-character bracketed paste (CSI 200~ <CR> CSI 201~): claude, codex
  // and bracketed-paste-aware shells (fish/zsh) insert pasted newlines literally
  // instead of executing the line. We capture the keydown on the container and
  // stopImmediatePropagation so xterm never also emits its default CR (a trailing
  // bare CR would submit the just-inserted newline).
  const onKeydown = (e: KeyboardEvent) => {
    if (e.key === "Enter" && e.shiftKey && !e.ctrlKey && !e.altKey && !e.metaKey) {
      e.preventDefault();
      e.stopImmediatePropagation();
      sendBytes(ws, "\x1b[200~\r\x1b[201~");
    }
  };
  if (!ro_) {
    node.addEventListener("keydown", onKeydown, true);
  }

  term.onData((d) => {
    // Drop xterm's auto-replies to terminal queries (CPR ESC[..R, Device
    // Attributes ESC[..c / ESC[?..c / ESC[>..c) and focus events (ESC[I / ESC[O).
    // In tmux control mode the inner program's queries are answered by tmux, so
    // these spurious replies would otherwise be echoed at the shell prompt.
    const clean = d.replace(/\x1b\[[?>]?[0-9;]*[Rc]|\x1b\[[IO]/g, "");
    if (clean) sendBytes(ws, clean);
  });

  const resizeObs = new ResizeObserver(() => { fitNow(); send(); });
  resizeObs.observe(node);

  // register a live theme applier so accent/monokai changes reach this terminal
  const applyTheme = (bg: string) => {
    term.options.theme = { ...term.options.theme, background: bg, cursorAccent: bg };
  };
  termThemes.add(applyTheme);

  return {
    destroy() {
      resizeObs.disconnect();
      ws.onclose = null;
      ws.close();
      term.dispose();
      if (!ro_) node.removeEventListener("keydown", onKeydown, true);
      termThemes.delete(applyTheme);
    },
  };
}
