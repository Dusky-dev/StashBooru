import { getClient } from "src/core/StashService";

type EntityAutoTagKind = "studio" | "performer" | "tag";

const refetchQueries: Record<EntityAutoTagKind, string> = {
  studio: "FindStudio",
  performer: "FindPerformer",
  tag: "FindTag",
};

export async function runEntityAutoTag(
  kind: EntityAutoTagKind,
  id: string
): Promise<void> {
  const response = await fetch(`/${kind}/${id}/auto-tag`, { method: "POST" });
  if (!response.ok) {
    throw new Error((await response.text()) || response.statusText);
  }

  await getClient().refetchQueries({ include: [refetchQueries[kind]] });
}
