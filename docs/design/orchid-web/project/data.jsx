/* ============ mock data + helpers ============ */

// Terminal line: { kind, text } — kind maps to a CSS class in .term-body
// kinds: cmd, dim, ok, warn, err, acc, agent, tool, "" (plain)

const claudeAuthLines = [
  { kind: "tool", text: "● Read src/router/index.ts (142 lines)" },
  { kind: "agent", text: "I'll add the new /worktrees route and wire it to the" },
  { kind: "agent", text: "session controller. Updating the router now." },
  { kind: "tool", text: "● Edit src/router/index.ts" },
  { kind: "dim",  text: "    + router.get('/worktrees/:id/sessions', listSessions)" },
  { kind: "tool", text: "● Bash  npm run typecheck" },
  { kind: "ok",   text: "  ✓ tsc — no errors (3.1s)" },
  { kind: "agent", text: "Typecheck passes. Want me to add a test for the" },
  { kind: "agent", text: "session listing endpoint as well?" },
];

const codexAuthLines = [
  { kind: "dim",  text: "thinking… inspecting auth middleware" },
  { kind: "tool", text: "$ grep -rn 'verifyToken' src/" },
  { kind: "dim",  text: "  src/mw/auth.ts:18  src/mw/auth.ts:54" },
  { kind: "agent", text: "Found two call sites. The refresh path skips the" },
  { kind: "agent", text: "expiry check — patching auth.ts:54." },
  { kind: "tool", text: "apply_patch src/mw/auth.ts" },
  { kind: "ok",   text: "  patched 1 file, +6 -2" },
  { kind: "tool", text: "$ npm test -- auth" },
  { kind: "ok",   text: "  PASS  12 passing (840ms)" },
];

const shellLines = [
  { kind: "cmd",  text: "git status", prompt: true },
  { kind: "dim",  text: "On branch feat/worktree-api" },
  { kind: "dim",  text: "Changes not staged for commit:" },
  { kind: "warn", text: "  modified:   src/router/index.ts" },
  { kind: "warn", text: "  modified:   src/mw/auth.ts" },
  { kind: "cmd",  text: "npm run dev", prompt: true },
  { kind: "acc",  text: "  ▲ dev server ready on http://localhost:5173" },
  { kind: "dim",  text: "  watching for file changes…" },
];

const idleClaude = [
  { kind: "agent", text: "Done — migration applied and all 24 tests pass." },
  { kind: "tool", text: "● Bash  git add -A && git commit -m 'feat: pool'" },
  { kind: "ok",   text: "  [feat/db-pool 9a1c2f] feat: connection pool" },
  { kind: "dim",  text: "" },
  { kind: "dim",  text: "Awaiting next instruction." },
];

const waitLines = [
  { kind: "agent", text: "This will delete 3 migration files and reset the" },
  { kind: "agent", text: "local database. This cannot be undone." },
  { kind: "dim",  text: "" },
  { kind: "warn", text: "  Proceed with reset? (y/n)" },
];

let _id = 100;
const uid = (p) => `${p}-${++_id}`;

function mkSession(type, name, status, mins, lines, task) {
  return {
    id: uid("s"),
    type, name, status,
    uptime: mins,
    lines: lines.slice(),
    task: task || null,
    input: "",
  };
}

const PROJECTS = [
  {
    id: "p-orchid",
    name: "orchid",
    path: "~/code/orchid",
    worktrees: [
      { id: "wt-main", name: "main", branch: "main", dirty: 0,
        sessions: [
          mkSession("claude", "router + sessions API", "working", 23, claudeAuthLines, "Wire /worktrees route"),
          mkSession("shell", "dev server", "working", 96, shellLines),
          mkSession("codex", "auth expiry bug", "wait", 8, waitLines, "Fix refresh token"),
          mkSession("claude", "docs update", "working", 6, [
            { kind: "tool", text: "● Read README.md" },
            { kind: "agent", text: "Updating the API reference for the new" },
            { kind: "agent", text: "/worktrees and /sessions endpoints." },
            { kind: "tool", text: "● Edit docs/api.md" },
          ], "Update API docs"),
          mkSession("shell", "logs tail", "idle", 210, [
            { kind: "cmd", text: "tail -f logs/app.log", prompt: true },
            { kind: "dim", text: "  [info] 200 GET /sessions  (4ms)" },
            { kind: "dim", text: "  [info] 200 GET /worktrees (2ms)" },
          ]),
        ] },
      { id: "wt-pool", name: "feat/db-pool", branch: "feat/db-pool", dirty: 4,
        sessions: [
          mkSession("claude", "connection pool", "idle", 41, idleClaude, "Add pg pool"),
          mkSession("shell", "psql", "idle", 30, [
            { kind: "cmd", text: "psql orchid_dev", prompt: true },
            { kind: "dim", text: "psql (16.2)" },
            { kind: "dim", text: "orchid_dev=#" },
          ]),
        ] },
      { id: "wt-ui", name: "feat/worktree-api", branch: "feat/worktree-api", dirty: 2,
        sessions: [
          mkSession("codex", "endpoint refactor", "working", 12, codexAuthLines, "Refactor list endpoint"),
          mkSession("shell", "test watch", "idle", 12, [
            { kind: "cmd", text: "npm run test:watch", prompt: true },
            { kind: "ok", text: "  ✓ 31 passing" },
            { kind: "dim", text: "  waiting for changes…" },
          ]),
        ] },
    ],
  },
  {
    id: "p-atlas",
    name: "atlas-api",
    path: "~/code/atlas-api",
    worktrees: [
      { id: "wt-a-main", name: "main", branch: "main", dirty: 0,
        sessions: [
          mkSession("codex", "rate limiter", "idle", 130, [
            { kind: "agent", text: "Rate limiter middleware added with Redis backend." },
            { kind: "ok", text: "  ✓ 18 passing" },
            { kind: "dim", text: "Awaiting next instruction." },
          ], "Add rate limiting"),
        ] },
      { id: "wt-a-mig", name: "chore/migrate-pg16", branch: "chore/migrate-pg16", dirty: 7,
        sessions: [] },
    ],
  },
  {
    id: "p-mono",
    name: "design-mono",
    path: "~/work/design-mono",
    worktrees: [
      { id: "wt-m-main", name: "main", branch: "main", dirty: 1,
        sessions: [
          mkSession("claude", "token export", "working", 4, [
            { kind: "tool", text: "● Read tokens/color.json" },
            { kind: "agent", text: "Generating CSS custom properties from the" },
            { kind: "agent", text: "color + spacing token sets…" },
            { kind: "tool", text: "● Write dist/tokens.css" },
            { kind: "ok", text: "  ✓ wrote 84 variables" },
          ], "Export design tokens"),
          mkSession("shell", "build", "idle", 4, [
            { kind: "cmd", text: "pnpm build", prompt: true },
            { kind: "ok", text: "  ✓ built in 1.9s" },
          ]),
        ] },
    ],
  },
];

// streaming fragments appended to "working" sessions over time, per type
const STREAM = {
  claude: [
    { kind: "tool", text: "● Read src/lib/parse.ts" },
    { kind: "agent", text: "Refactoring the parser to handle nested blocks." },
    { kind: "tool", text: "● Edit src/lib/parse.ts" },
    { kind: "tool", text: "● Bash  npm run lint" },
    { kind: "ok",   text: "  ✓ lint clean" },
    { kind: "agent", text: "That's resolved. Moving to the next item." },
  ],
  codex: [
    { kind: "dim",  text: "thinking… mapping call graph" },
    { kind: "tool", text: "$ rg 'export function' src/" },
    { kind: "agent", text: "Updating the signature across 3 callers." },
    { kind: "tool", text: "apply_patch src/index.ts" },
    { kind: "ok",   text: "  patched 3 files, +14 -9" },
  ],
  shell: [
    { kind: "dim",  text: "  hmr update /src/app.jsx" },
    { kind: "dim",  text: "  page reload src/styles.css" },
    { kind: "acc",  text: "  ✓ recompiled in 142ms" },
  ],
};

const AGENT_META = {
  claude: { label: "Claude", badge: "C", cmd: "claude" },
  codex:  { label: "Codex",  badge: "Cx", cmd: "codex" },
  shell:  { label: "Shell",  badge: ">_", cmd: "zsh" },
};

const STATUS_META = {
  working: { label: "Running", cls: "ok", pulse: true },
  idle:    { label: "Idle", cls: "idle", pulse: false },
  wait:    { label: "Waiting", cls: "wait", pulse: true },
};

function fmtUptime(mins) {
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m`;
  const h = Math.floor(mins / 60), m = mins % 60;
  return m ? `${h}h ${m}m` : `${h}h`;
}

Object.assign(window, { PROJECTS, STREAM, AGENT_META, STATUS_META, fmtUptime, mkSession, uid, MAX_GRID: 9 });
