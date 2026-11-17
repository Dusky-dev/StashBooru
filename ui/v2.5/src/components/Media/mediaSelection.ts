// Native numeric IDs are scoped to a media type. Keep the typed key at every
// selection boundary and only remove the prefix when dispatching native APIs.
export interface MediaSelectionItem {
  id: string;
  image?: { id: string } | null;
  scene?: { id: string } | null;
}

export function splitMediaSelection(items: readonly MediaSelectionItem[]) {
  const imageIDs: string[] = [];
  const sceneIDs: string[] = [];
  for (const item of items) {
    if (item.image && item.id === `image:${item.image.id}`)
      imageIDs.push(item.image.id);
    else if (item.scene && item.id === `scene:${item.scene.id}`)
      sceneIDs.push(item.scene.id);
    else throw new Error(`Invalid media selection: ${item.id}`);
  }
  return { imageIDs: [...new Set(imageIDs)], sceneIDs: [...new Set(sceneIDs)] };
}

export function mediaConversionTargets(items: readonly MediaSelectionItem[]) {
  const { imageIDs, sceneIDs } = splitMediaSelection(items);
  return [
    ...imageIDs.map((id) => ({ kind: "image" as const, id: Number(id) })),
    ...sceneIDs.map((id) => ({ kind: "scene" as const, id: Number(id) })),
  ];
}

export async function dispatchMediaUpdates<T>(
  items: readonly MediaSelectionItem[],
  input: T,
  updateImages: (ids: string[], input: T) => Promise<unknown>,
  updateScenes: (ids: string[], input: T) => Promise<unknown>
) {
  const { imageIDs, sceneIDs } = splitMediaSelection(items);
  // Each native mutation retains its validation, hooks and transaction rules.
  // Report both outcomes so a partial success is visible and can be retried.
  const update = async (
    ids: string[],
    write: (ids: string[], input: T) => Promise<unknown>
  ) => {
    if (ids.length > 0) return write(ids, input);
  };
  return Promise.allSettled([
    update(imageIDs, updateImages),
    update(sceneIDs, updateScenes),
  ]);
}

export function summarizeMediaUpdates(
  items: readonly MediaSelectionItem[],
  outcomes: readonly PromiseSettledResult<unknown>[]
) {
  const { imageIDs, sceneIDs } = splitMediaSelection(items);
  const confirmed: string[] = [];
  const errors: string[] = [];
  const groups = [
    { ids: imageIDs, kind: "image", label: "Images", outcome: outcomes[0] },
    { ids: sceneIDs, kind: "scene", label: "Videos", outcome: outcomes[1] },
  ];
  for (const { ids, kind, label, outcome } of groups) {
    if (ids.length === 0) continue;
    if (outcome?.status === "rejected") {
      const reason = outcome.reason;
      errors.push(
        `${label}: ${reason instanceof Error ? reason.message : String(reason)}`
      );
      continue;
    }
    const returned = new Set<string>(
      Array.isArray(outcome?.value) ? outcome.value : []
    );
    const saved = ids.filter((id) => returned.has(id));
    confirmed.push(...saved.map((id) => `${kind}:${id}`));
    const missing = ids.length - saved.length;
    if (missing > 0) {
      errors.push(`${label}: Stash did not confirm ${missing} of ${ids.length}.`);
    }
  }
  return { confirmed, errors };
}
