// Pause is observed between whole imports, after the active item finishes.
export async function runUploadBatch<T>(
  items: readonly T[],
  shouldPause: () => boolean,
  upload: (item: T, hasMore: boolean) => Promise<boolean>
) {
  for (const [index, item] of items.entries()) {
    if (shouldPause()) return;
    if (!(await upload(item, index + 1 < items.length))) return;
  }
}
