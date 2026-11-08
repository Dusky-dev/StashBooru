export const defaultDuplicateMethod = "phash" as const;
export type DuplicateMatchMethod = "phash" | "embedding";

export function duplicateClusterURL(
  method: DuplicateMatchMethod,
  distance: string,
  threshold: string
): string {
  const params = new URLSearchParams({ method });
  if (method === "phash") {
    params.set("distance", distance);
  } else {
    params.set("threshold", threshold);
    params.set("neighbors", "20");
  }
  return "image/visual-similarity/clusters?" + params.toString();
}

export interface SimilarImageMember {
  id: number;
  width: number;
  height: number;
  fileSize: number;
  preferred: boolean;
}

export interface SimilarImageGroup {
  ids: number[];
  members: SimilarImageMember[];
  preferredID: number;
}

/** Start a pair with distinct images, keeping the server suggestion as reference. */
export function duplicateComparisonPair(
  group: SimilarImageGroup,
  clickedID: number
): { referenceID: number; selectedID: number } {
  const ids = group.members.map((member) => member.id);
  const referenceID = ids.includes(group.preferredID)
    ? group.preferredID
    : ids[0];
  const selectedID =
    clickedID !== referenceID && ids.includes(clickedID)
      ? clickedID
      : (ids.find((id) => id !== referenceID) ?? referenceID);
  return { referenceID, selectedID };
}

export function duplicateGroupKey(group: SimilarImageGroup): string {
  return [...group.ids].sort((a, b) => a - b).join(":");
}

export function keptImageIDs(
  group: SimilarImageGroup,
  selected?: number[]
): number[] {
  const members = new Set(group.members.map((member) => member.id));
  const kept = (selected ?? [group.preferredID]).filter((id) =>
    members.has(id)
  );
  return kept.length ? [...new Set(kept)] : [group.members[0].id];
}

export function duplicateReviewPlan(
  group: SimilarImageGroup,
  selected?: number[]
): { keep: number[]; remove: number[] } {
  const keep = keptImageIDs(group, selected);
  const kept = new Set(keep);
  return {
    keep,
    remove: group.members
      .map((member) => member.id)
      .filter((id) => !kept.has(id)),
  };
}

/** Build a safe add-only metadata transfer independent of deletion selections. */
export function similarityMetadataCopyPlan(
  group: SimilarImageGroup,
  selectedSourceID: number,
  selectedTargetIDs: number[]
): { sourceImageID: number; targetImageIDs: number[] } {
  const memberIDs = group.members.map((member) => member.id);
  const sourceImageID = memberIDs.includes(selectedSourceID)
    ? selectedSourceID
    : memberIDs.includes(group.preferredID)
      ? group.preferredID
      : (memberIDs[0] ?? 0);
  const targetImageIDs = [
    ...new Set(
      selectedTargetIDs.filter(
        (id) => memberIDs.includes(id) && id !== sourceImageID
      )
    ),
  ];
  return { sourceImageID, targetImageIDs };
}

export function readableFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "Unknown size";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(0) + " KiB";
  return (bytes / (1024 * 1024)).toFixed(2) + " MiB";
}
