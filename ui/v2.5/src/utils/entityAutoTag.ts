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

  // Copyright explicitly refetches the live detail query after auto-tagging.
  // Do the same for the shared Artist / Character / Tag path by refetching the
  // queries that are actually active on the current page, rather than relying
  // on fragile operation-name strings that may not match the mounted query.
  await getClient().refetchQueries({ include: "active" });
}
