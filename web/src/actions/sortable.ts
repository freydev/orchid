import Sortable from 'sortablejs';

export type SortableParams = {
  handle?: string;
  itemSel: string;
  getId: (el: HTMLElement) => string;
  onOrder: (ids: string[]) => void;
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
      onEnd: () =>
        p.onOrder(
          [...node.querySelectorAll(':scope > ' + p.itemSel)].map((el) => p.getId(el as HTMLElement)),
        ),
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
