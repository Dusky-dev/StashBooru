export interface BooruMediaCountSource {
  scene_count?: number | null;
  image_count?: number | null;
}

function validCount(value: number | null | undefined): number | null {
  return typeof value === "number" &&
    Number.isSafeInteger(value) &&
    value >= 0
    ? value
    : null;
}

/** Treat an absent GraphQL field as unknown, never as a real zero count. */
export function booruEntityMediaCounts(entity: BooruMediaCountSource) {
  return {
    videos: validCount(entity.scene_count),
    images: validCount(entity.image_count),
  };
}
