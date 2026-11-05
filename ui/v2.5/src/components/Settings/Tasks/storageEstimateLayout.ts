// P12 returns a string layout; P13 returns an algorithm object such as
// { kind: "independent" } or { kind: "fixed", size: 65536 }.
// Do not render raw API objects as React children (React error #31).
export function formatStorageEstimateLayout(layout: unknown): string {
  if (typeof layout === "string") return layout || "Unknown";

  if (layout !== null && typeof layout === "object" && !Array.isArray(layout)) {
    const candidate = layout as Record<string, unknown>;
    const { kind, size } = candidate;
    if (typeof kind === "string" && kind.length > 0) {
      if (typeof size === "number" && Number.isSafeInteger(size) && size > 0)
        return `${kind}, size ${size}`;
      return kind;
    }
  }
  return "Unknown";
}
