/* terminal: Svelte action wrapping xterm + FitAddon + WebSocket.
   Ported from web/app.js `attachInto`. */

import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import "@xterm/xterm/css/xterm.css";
import { MONOKAI, termThemes } from "../lib/theme";
import { openSessionSocket, sendResize, sendBytes } from "../lib/ws";
import { registerTerminal, unregisterTerminal } from "../lib/terminalInput";

export interface TerminalParams {
  id: string;
}

/* GPU renderer. Skipped on touch / narrow (mobile) layouts: the mobile touch
   handling below relies on native browser selection of xterm's DOM rows
   (base.css), which a canvas renderer has no text for. On WebGL context loss
   (browsers cap live contexts, ~16) the addon is disposed and xterm falls back
   to its DOM renderer. */
function loadWebgl(term: Terminal): () => void {
  if (matchMedia("(pointer: coarse), (max-width: 640px)").matches) return () => {};
  try {
    const gl = new WebglAddon();
    gl.onContextLoss(() => gl.dispose());
    term.loadAddon(gl);
    return () => gl.dispose();
  } catch {
    return () => {}; // no WebGL2 -> DOM renderer
  }
}

export function terminal(node: HTMLElement, params: TerminalParams): { destroy(): void } {
  const { id } = params;
  const term = new Terminal({
    cursorBlink: true,
    fontSize: 13.5,
    fontFamily: '"Fira Mono", "JetBrains Mono", ui-monospace, Menlo, monospace',
    scrollback: 5000,
    theme: MONOKAI,
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(node);
  const disposeWebgl = loadWebgl(term);

  const fitNow = () => { fit.fit(); };
  fitNow();

  const ws = openSessionSocket(id);

  const send = () => { sendResize(ws, term.cols, term.rows); };
  ws.onopen = () => send();
  ws.onmessage = (e) => {
    if (e.data instanceof ArrayBuffer) { term.write(new Uint8Array(e.data)); return; }
    // string frames are control messages (pane geometry); the terminal owns its own size
  };
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
  // ---- mobile touch handling ----
  // xterm's screen overlays the (empty) scroll viewport and runs its OWN
  // selection + scroll on touch, fighting the browser: it preventDefaults the
  // native long-press selection and draws a second, jumpy selection overlay,
  // while our scroll dragged the viewport mid-selection. We take over fully:
  //  - a swipe (>threshold move) scrolls the viewport pixel-1:1 with inertia;
  //  - a long-press (no early move) enters SELECT mode: we stop scrolling so the
  //    browser's native selection isn't yanked around;
  //  - a quick tap focuses the terminal (opens the keyboard);
  //  - xterm's own pointer/mouse handlers are suppressed for touch so only the
  //    native selection happens (see user-select / hidden .xterm-selection in
  //    base.css).
  const viewport = () => node.querySelector(".xterm-viewport") as HTMLElement | null;
  let touching = false;
  let mode: "" | "scroll" | "select" = "";
  let startX = 0, startY = 0, lastY = 0, lastT = 0, vel = 0;
  let pressTimer = 0, glideRAF = 0;
  const MOVE_THRESH = 10;   // px before a drag counts as a scroll
  const LONGPRESS_MS = 350; // hold this long without moving -> selection
  const stopGlide = () => { if (glideRAF) { cancelAnimationFrame(glideRAF); glideRAF = 0; } };
  const scrollBy = (vp: HTMLElement, dy: number): boolean => {
    const before = vp.scrollTop;
    vp.scrollTop = before + dy;
    return vp.scrollTop !== before;
  };
  const onTouchStart = (e: TouchEvent) => {
    if (e.touches.length !== 1) return;
    e.stopImmediatePropagation(); // keep xterm out of it
    stopGlide();
    touching = true;
    mode = "";
    const t = e.touches[0];
    startX = t.clientX; startY = t.clientY; lastY = t.clientY; lastT = e.timeStamp; vel = 0;
    clearTimeout(pressTimer);
    pressTimer = window.setTimeout(() => { if (mode === "") mode = "select"; }, LONGPRESS_MS);
  };
  const onTouchMove = (e: TouchEvent) => {
    if (e.touches.length !== 1) return;
    e.stopImmediatePropagation();
    const t = e.touches[0];
    if (mode === "" && Math.hypot(t.clientX - startX, t.clientY - startY) > MOVE_THRESH) {
      mode = "scroll";
      clearTimeout(pressTimer);
    }
    if (mode !== "scroll") return; // select / undecided: let native selection run
    const vp = viewport();
    if (!vp) return;
    const dy = lastY - t.clientY;
    vel = dy / ((e.timeStamp - lastT) || 16);
    lastY = t.clientY; lastT = e.timeStamp;
    if (scrollBy(vp, dy) && e.cancelable) e.preventDefault();
  };
  const onTouchEnd = () => {
    clearTimeout(pressTimer);
    if (mode === "scroll") {
      const vp = viewport();
      let v = vel * 16; // px per frame at ~60fps
      if (vp && Math.abs(v) >= 1.5) {
        const glide = () => {
          if (Math.abs(v) < 0.4 || !scrollBy(vp, v)) { glideRAF = 0; return; }
          v *= 0.95;
          glideRAF = requestAnimationFrame(glide);
        };
        glideRAF = requestAnimationFrame(glide);
      }
    } else if (mode === "") {
      term.focus(); // quick tap, no selection -> open the keyboard
    }
    setTimeout(() => { touching = false; }, 0); // after synthesized mouse events
  };
  // Stop xterm's pointer/mouse selection for touch-originated events (real mouse
  // on desktop still works: pointerType 'mouse' / not touching -> passed through).
  const swallowForTouch = (e: Event) => {
    const pe = e as PointerEvent;
    if (pe.pointerType === "mouse") return;
    if (pe.pointerType === "touch" || touching) e.stopImmediatePropagation();
  };

  node.addEventListener("keydown", onKeydown, true);
  node.addEventListener("touchstart", onTouchStart, { capture: true, passive: true });
  node.addEventListener("touchmove", onTouchMove, { capture: true, passive: false });
  node.addEventListener("touchend", onTouchEnd, { capture: true, passive: true });
  node.addEventListener("pointerdown", swallowForTouch, true);
  node.addEventListener("mousedown", swallowForTouch, true);
  // expose an input sender so the mobile status-bar arrow buttons can inject
  // keys into this (writable, focused) session.
  registerTerminal(id, (d) => sendBytes(ws, d));

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

  // When the page is opened in a not-yet-active window (e.g. launched from a
  // desktop notification / PWA protocol handler), xterm can initialize before
  // the window is focused or laid out, leaving keyboard input dead until a
  // reload. Re-fit and re-focus once the window actually becomes visible.
  const onActivate = () => {
    if (document.hidden) return;
    fitNow();
    send();
    term.focus();
  };
  document.addEventListener("visibilitychange", onActivate);
  window.addEventListener("focus", onActivate);
  requestAnimationFrame(onActivate); // settle the just-mounted case

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
      disposeWebgl();
      term.dispose();
      stopGlide();
      clearTimeout(pressTimer);
      document.removeEventListener("visibilitychange", onActivate);
      window.removeEventListener("focus", onActivate);
      node.removeEventListener("keydown", onKeydown, true);
      node.removeEventListener("touchstart", onTouchStart, true);
      node.removeEventListener("touchmove", onTouchMove, true);
      node.removeEventListener("touchend", onTouchEnd, true);
      node.removeEventListener("pointerdown", swallowForTouch, true);
      node.removeEventListener("mousedown", swallowForTouch, true);
      unregisterTerminal(id);
      termThemes.delete(applyTheme);
    },
  };
}
