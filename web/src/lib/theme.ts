/* theme: accent / monokai / rail width — ported from web/app.js */

export interface DotColor { id: string; hex: string }

// curated accent colors for projects/sessions (mirrors the backend palette)
export const DOT_COLORS: DotColor[] = [
  { id: "blue", hex: "#7c9cff" }, { id: "green", hex: "#5fd39a" },
  { id: "amber", hex: "#e9a23b" }, { id: "purple", hex: "#c98bdb" },
  { id: "red", hex: "#e57373" }, { id: "cyan", hex: "#5fc8d3" },
  { id: "pink", hex: "#e88bb0" }, { id: "slate", hex: "#8a93a6" },
  { id: "monokai", hex: "#f92672" },
];

export const colorHex = (id?: string | null): string | null =>
  (DOT_COLORS.find((c) => c.id === id) || ({} as Partial<DotColor>)).hex || null;

export const DEFAULT_ACCENT = "purple";

export interface XtermTheme {
  background: string; foreground: string; cursor: string; cursorAccent: string;
  selectionBackground: string;
  black: string; brightBlack: string; red: string; brightRed: string;
  green: string; brightGreen: string; yellow: string; brightYellow: string;
  blue: string; brightBlue: string; magenta: string; brightMagenta: string;
  cyan: string; brightCyan: string; white: string; brightWhite: string;
}

export const MONOKAI: XtermTheme = {
  background: "#202020", foreground: "#f8f8f2", cursor: "#a6e22e", cursorAccent: "#202020",
  selectionBackground: "rgba(232,139,176,0.30)",
  black: "#272822", brightBlack: "#75715e", red: "#f92672", brightRed: "#f92672",
  green: "#a6e22e", brightGreen: "#a6e22e", yellow: "#e6db74", brightYellow: "#f4bf75",
  blue: "#66d9ef", brightBlue: "#66d9ef", magenta: "#ae81ff", brightMagenta: "#ae81ff",
  cyan: "#a1efe4", brightCyan: "#a1efe4", white: "#f8f8f2", brightWhite: "#f9f8f5",
};

// registry of live terminal theme appliers; setTermBg re-themes all open terminals
export const termThemes = new Set<(bg: string) => void>();
function setTermBg(bg: string) {
  MONOKAI.background = bg; MONOKAI.cursorAccent = bg;
  for (const apply of termThemes) apply(bg);
}

// apply an accent color id to the live CSS variables (derives dim + bg tints)
// Re-tint the whole TUI palette toward the accent hue: surfaces, borders and
// text are gray levels mixed toward the accent (the "phosphor" cast follows the
// accent), while wait/idle/danger stay semantic.
// canonical Monokai Classic palette — warm olive surfaces + pink accent, not a
// single-hue tint (that's what makes it look like monokai).
export function applyMonokai() {
  const root = document.documentElement.style;
  const v: Record<string, string> = {
    "--bg": "#1e1f1c", "--surface": "#272822", "--surface-2": "#2e2f28", "--surface-3": "#383a32",
    "--border": "#41423b", "--border-2": "#565849",
    "--text": "#f8f8f2", "--text-dim": "#a59f85", "--text-faint": "#75715e",
    "--accent": "#f92672", "--accent-dim": "#c01f5b",
    "--accent-bg": "rgba(249,38,114,0.14)", "--accent-bg-2": "rgba(249,38,114,0.22)",
    "--ok": "#a6e22e", "--ok-bg": "rgba(166,226,46,0.14)",
    "--wait": "#e6db74", "--wait-bg": "rgba(230,219,116,0.15)",
    "--idle": "#75715e", "--danger": "#f92672",
    "--term-bg": "#32332b",
  };
  for (const k in v) root.setProperty(k, v[k]);
  setTermBg("#32332b");
}

export function applyAccent(id: string) {
  if (id === "monokai") { applyMonokai(); return; }
  const hex = colorHex(id) || colorHex(DEFAULT_ACCENT)!;
  const h = hex.replace("#", "");
  const r = parseInt(h.slice(0, 2), 16), g = parseInt(h.slice(2, 4), 16), b = parseInt(h.slice(4, 6), 16);
  const root = document.documentElement.style;
  const hx = (a: number[]) => "#" + a.map((x) => Math.max(0, Math.min(255, Math.round(x))).toString(16).padStart(2, "0")).join("");
  // mix a neutral gray `level` toward the accent by weight w (0..1)
  const tint = (level: number, w: number) => hx([level + (r - level) * w, level + (g - level) * w, level + (b - level) * w]);

  root.setProperty("--accent", hex);
  root.setProperty("--accent-dim", hx([r * 0.72, g * 0.72, b * 0.72]));
  root.setProperty("--accent-bg", "rgba(" + r + "," + g + "," + b + ",0.13)");
  root.setProperty("--accent-bg-2", "rgba(" + r + "," + g + "," + b + ",0.20)");
  // light/calm theme: cast the neutral surfaces+text gently toward the accent
  // hue (lighter base levels + low weights keep it soft, not phosphor-heavy).
  root.setProperty("--bg", tint(24, 0.05));
  root.setProperty("--surface", tint(31, 0.06));
  root.setProperty("--surface-2", tint(39, 0.08));
  root.setProperty("--surface-3", tint(49, 0.11));
  root.setProperty("--border", tint(49, 0.14));
  root.setProperty("--border-2", tint(66, 0.20));
  root.setProperty("--text", tint(230, 0.05));
  root.setProperty("--text-dim", tint(160, 0.12));
  root.setProperty("--text-faint", tint(108, 0.16));
  root.setProperty("--ok", hex);
  root.setProperty("--ok-bg", "rgba(" + r + "," + g + "," + b + ",0.13)");

  // terminal background: a soft dark neutral with a faint accent cast
  const t = (c: number) => Math.round(16 + c * 0.05);
  const termbg = hx([t(r), t(g), t(b)]);
  root.setProperty("--term-bg", termbg);
  setTermBg(termbg);
}

/* ---------- Terminal sidebar (rail) resizing ---------- */
export const RAIL_MIN = 180, RAIL_MAX = 520;
let railWidth = 230;
export function applyRailWidth(px: number) {
  railWidth = Math.max(RAIL_MIN, Math.min(RAIL_MAX, Math.round(px) || 230));
  document.documentElement.style.setProperty("--rail-w", railWidth + "px");
}
