// modal store + confirm mechanism
export const modal = $state({ kind: null as string | null, props: {} as any });
export const openModal = (k: string, p: any = {}) => { modal.kind = k; modal.props = p; };
export const closeModal = () => { modal.kind = null; };

// ---- confirm dialog ----
let confirmResolve: ((v: boolean) => void) | null = null;
export const confirmState = $state({ open: false, title: '', body: '', confirmLabel: 'Confirm', danger: true });
export function confirm(opts: { title: string; body: string; confirmLabel?: string; danger?: boolean }): Promise<boolean> {
  return new Promise(res => {
    confirmResolve = res;
    Object.assign(confirmState, { open: true, danger: true, confirmLabel: 'Confirm', ...opts });
  });
}
export function resolveConfirm(v: boolean) {
  confirmState.open = false;
  const r = confirmResolve;
  confirmResolve = null;
  r?.(v);
}
