/* Registry of writable terminals' input senders, keyed by session id. The
   terminal action registers a sender on mount (focus view only — read-only
   preview panes don't register), so UI outside the terminal (e.g. the mobile
   arrow-key buttons in the status bar) can inject bytes into the active
   session. */

const senders = new Map<string, (data: string) => void>();

export function registerTerminal(id: string, send: (data: string) => void) {
  senders.set(id, send);
}

export function unregisterTerminal(id: string) {
  senders.delete(id);
}

// send bytes to the given session's terminal, if it is currently attached
export function sendToTerminal(id: string | null | undefined, data: string) {
  if (id) senders.get(id)?.(data);
}
