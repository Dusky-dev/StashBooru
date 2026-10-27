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

export function duplicateGroupKey(group: SimilarImageGroup): string {
  return [...group.ids].sort((a, b) => a - b).join(":");
}

export function keptImageIDs(
  group: SimilarImageGroup,
  selected?: number[]
): number[] {
  const members = new Set(group.members.map((member) => member.id));
  const kept = (selected ?? [group.preferredID]).filter((id) => members.has(id));
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
    remove: group.members.map((member) => member.id).filter((id) => !kept.has(id)),
  };
}

export function readableFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "Unknown size";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(0) + " KiB";
  return (bytes / (1024 * 1024)).toFixed(2) + " MiB";
}
