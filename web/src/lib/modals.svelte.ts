// minimal modal store — modals render in a later task; for now these are safe no-op-able setters
export const modal = $state({ kind: null as string | null, props: {} as any });
export const openModal = (k: string, p: any = {}) => { modal.kind = k; modal.props = p; };
export const closeModal = () => { modal.kind = null; };
