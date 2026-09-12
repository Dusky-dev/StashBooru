import { getClient } from "src/core/StashService";

type EntityAutoTagKind = "studio" | "performer" | "tag";

export async function runEntityAutoTag(
  kind: EntityAutoTagKind,
  id: string
): Promise<void> {
  const response = await fetch(`/${kind}/${id}/auto-tag`, { method: "POST" });
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }

  // Copyright auto-tag refetches its live detail query after the synchronous
  // matcher finishes. Do the same for every first-class entity instead of
  // relying on a query-name lookup that can leave the currently mounted page
  // rendering stale fragment data.
  await getClient().refetchQueries({ include: "active" });
}
