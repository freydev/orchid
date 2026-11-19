import Sortable from 'sortablejs';

export type SortableParams = {
  handle?: string;
  itemSel: string;
  getId: (el: HTMLElement) => string;
  onOrder: (ids: string[]) => void;
  // lists sharing a group name accept each other's items; a cross-list drop is
  // reverted in the DOM (Svelte owns it) and reported via onTransfer instead
  group?: string;
  onTransfer?: (id: string, to: HTMLElement) => void;
};

export function sortable(node: HTMLElement, params: SortableParams) {
  let s: Sortable | null = null;
  function create(p: SortableParams) {
    s = Sortable.create(node, {
      draggable: p.itemSel,
      handle: p.handle,
      animation: 140,
      ghostClass: 'drag-ghost',
      fallbackOnBody: true,
      group: p.group,
      onEnd: (e) => {
        if (e.from !== e.to) {
          e.from.insertBefore(e.item, e.from.children[e.oldIndex ?? 0] ?? null);
          p.onTransfer?.(p.getId(e.item), e.to);
          return;
        }
        p.onOrder(
          [...node.querySelectorAll(':scope > ' + p.itemSel)].map((el) => p.getId(el as HTMLElement)),
        );
      },
    });
  }
  create(params);
  return {
    update(p: SortableParams) {
      // options rarely change; recreate to be safe
      s?.destroy();
      create(p);
    },
    destroy() {
      s?.destroy();
      s = null;
    },
  };
}
