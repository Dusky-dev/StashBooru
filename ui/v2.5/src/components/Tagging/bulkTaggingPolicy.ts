/**
 * An opt-in bulk job is only valid for an explicit, image-only multi-selection.
 * Keep typed Image/Video IDs separate (particularly on the All page).
 */
export interface BulkReviewItem {
  kind: string;
  id: string;
}

export interface BulkTaggingConfig {
  bulkTaggingEnabled: boolean;
  threshold: number;
  limit: number;
}

export interface BulkTaggingCategories {
  applyCharacters: boolean;
  applyArtist: boolean;
  applyCopyright: boolean;
  applyTags: boolean;
}

export const defaultBulkTaggingCategories: BulkTaggingCategories = {
  applyCharacters: true,
  applyArtist: true,
  applyCopyright: true,
  applyTags: true,
};

export function bulkSelectedImageIDs(
  selection: readonly BulkReviewItem[],
  imageKind: string
): number[] | undefined {
  if (
    selection.length < 2 ||
    selection.some((item) => item.kind !== imageKind)
  ) {
    return undefined;
  }

  const ids = selection.map((item) => {
    if (!/^[1-9]\d*$/.test(item.id)) return NaN;
    return Number(item.id);
  });
  if (ids.some((id) => !Number.isSafeInteger(id))) return undefined;
  return ids;
}

export function bulkTaggingRequest(
  imageIDs: number[],
  config: BulkTaggingConfig,
  categories: BulkTaggingCategories
) {
  return {
    imageIDs,
    all: false,
    threshold: config.threshold,
    limit: config.limit,
    ...categories,
    // Keep existing Artist links; never replace them as a bulk default.
    replaceArtist: false,
  };
}
