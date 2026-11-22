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
  filenameEnabled: boolean;
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
  useFilename: boolean;
  useCamie: boolean;
  useEva02: boolean;
}

// Every allowed source starts checked. Local filename metadata needs only
// the job confirmation; Camie and EVA02
// stay behind the bulk model-tagging opt-in in tagging setup.
export function availableBulkTaggingSources(
  config: Pick<BulkTaggingConfig, "bulkTaggingEnabled" | "filenameEnabled">
): BulkTaggingSources {
  return {
    useFilename: config.filenameEnabled,
    useCamie: config.bulkTaggingEnabled,
    useEva02: config.bulkTaggingEnabled,
  };
}

/** Keep only the sources the settings allow, so a stale choice is never sent. */
export function allowedBulkTaggingSources(
  sources: BulkTaggingSources,
  config: Pick<BulkTaggingConfig, "bulkTaggingEnabled" | "filenameEnabled">
): BulkTaggingSources {
  const available = availableBulkTaggingSources(config);
  return {
    useFilename: sources.useFilename && available.useFilename,
    useCamie: sources.useCamie && available.useCamie,
    useEva02: sources.useEva02 && available.useEva02,
  };
}

export type TaggingActionKind = "image" | "video" | "mixed";

/** Name the tagging action after the media it will tag. */
export function taggingActionKind(
  items: readonly BulkReviewItem[],
  imageKind: string
): TaggingActionKind {
  const images = items.filter((item) => item.kind === imageKind).length;
  if (images === items.length) return "image";
  if (images === 0) return "video";
  return "mixed";
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
    ...allowedBulkTaggingSources(sources, config),
    ...categories,
    // Keep existing Artist links; never replace them as a bulk default.
    replaceArtist: false,
  };
}
