/* railResize: Svelte action for the terminal-view sidebar resizer.
   Ported from web/app.js `startRailResize` + `persistRailWidth`. */

import { applyRailWidth, RAIL_MIN, RAIL_MAX } from "../lib/theme";
import { app } from "../lib/state.svelte";
import { api } from "../lib/api";

// persist a new rail width: update the settings cache + the server
export function persistRailWidth(px: number) {
  const w = Math.round(px);
  app.settings.rail_width = w;
  api.railWidth(w).catch(() => {});
}

export function railResize(node: HTMLElement): { destroy(): void } {
  let railWidth = RAIL_MIN;

  function onMousedown(e: MouseEvent) {
    const view = node.closest(".focus-view") as HTMLElement | null;
    if (!view) return;
    e.preventDefault();
    const cs = getComputedStyle(view);
    const left = view.getBoundingClientRect().left + (parseFloat(cs.paddingLeft) || 0);
    node.classList.add("dragging");
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";

    const move = (ev: MouseEvent) => {
      railWidth = Math.max(RAIL_MIN, Math.min(RAIL_MAX, Math.round(ev.clientX - left) || 230));
      applyRailWidth(railWidth);
    };
    const up = () => {
      document.removeEventListener("mousemove", move);
      document.removeEventListener("mouseup", up);
      node.classList.remove("dragging");
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      // the xterm ResizeObserver reflows panes to the new width automatically
      persistRailWidth(railWidth);
    };
    document.addEventListener("mousemove", move);
    document.addEventListener("mouseup", up);
  }

  node.addEventListener("mousedown", onMousedown);
  return {
    destroy() {
      node.removeEventListener("mousedown", onMousedown);
    },
  };
}
