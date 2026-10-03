export type StackRef = { kind: "IMAGE" | "VIDEO"; id: string };
export const stackKey = (media: StackRef) =>
  `${media.kind === "IMAGE" ? "image" : "scene"}:${media.id}`;
export const stackPath = (media: StackRef) =>
  `/${media.kind === "IMAGE" ? "images" : "scenes"}/${media.id}`;

export function parseMemberURL(value: string): StackRef | undefined {
  try {
    const path = new URL(value.trim(), "https://stash.invalid").pathname;
    const match = /^\/(images|scenes)\/([1-9]\d*)\/?$/.exec(path);
    if (!match || !Number.isSafeInteger(Number(match[2]))) return undefined;
    return { kind: match[1] === "images" ? "IMAGE" : "VIDEO", id: match[2] };
  } catch {
    return undefined;
  }
}

export function nextStackMember(
  ordered: string[],
  selected: string,
  direction: -1 | 1
) {
  if (ordered.length === 0) return undefined;
  const index = Math.max(0, ordered.indexOf(selected));
  return ordered[(index + direction + ordered.length) % ordered.length];
}
