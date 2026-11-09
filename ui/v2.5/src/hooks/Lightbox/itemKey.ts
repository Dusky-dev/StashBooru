import type { ILightboxImage } from "./types";

// A mixed carousel can contain an Image and a Video with the same native ID,
// and Videos can have no screenshot. Path-only React keys collide in both cases.
export function lightboxEntryKey(item: ILightboxImage, index: number): string {
  const ref = item.mediaReference;
  if (ref) return `${ref.kind}:${ref.id}`;
  if (item.id) return `IMAGE:${item.id}`;
  if (item.href) return `href:${item.href}`;
  return `slide:${index}:${item.paths.image ?? item.paths.thumbnail ?? ""}`;
}
