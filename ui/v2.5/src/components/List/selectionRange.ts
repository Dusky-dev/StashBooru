// Ranges belong to the visible page. Selection may retain items from other
// pages, so an anchor removed by paging/filtering starts at the first item.
export function selectionRange<T extends { id: string }>(
  items: T[],
  anchorID: string | undefined,
  targetID: string
): T[] {
  const target = items.findIndex((item) => item.id === targetID);
  if (target === -1) return [];

  const anchor = Math.max(
    0,
    items.findIndex((item) => item.id === anchorID)
  );
  return items.slice(Math.min(anchor, target), Math.max(anchor, target) + 1);
}
