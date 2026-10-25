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
  eva02Threshold: number;
  limit: number;
}

export interface BulkTaggingCategories {
  applyCharacters: boolean;
  applyArtist: boolean;
  applyCopyright: boolean;
  applyTags: boolean;
}

export interface BulkTaggingSources {
  useCamie: boolean;
  useEva02: boolean;
}

export const defaultBulkTaggingSources: BulkTaggingSources = {
  useCamie: true,
  useEva02: true,
};

export const MANUAL_BULK_WARNING_KEY =
  "stashbooru:manual-bulk-tagging-warning-dismissed:v1";

export function manualBulkWarningDismissed(storage?: Pick<Storage, "getItem">) {
  try {
    return storage?.getItem(MANUAL_BULK_WARNING_KEY) === "1";
  } catch {
    // Private browsing can block storage; fail safely by showing the warning.
    return false;
  }
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
  categories: BulkTaggingCategories,
  sources: BulkTaggingSources
) {
  return {
    imageIDs,
    all: false,
    confirmed: true,
    threshold: config.threshold,
    eva02Threshold: config.eva02Threshold,
    limit: config.limit,
    ...sources,
    ...categories,
    // Keep existing Artist links; never replace them as a bulk default.
    replaceArtist: false,
  };
}
