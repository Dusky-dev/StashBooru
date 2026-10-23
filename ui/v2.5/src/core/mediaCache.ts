import type { ApolloCache } from "@apollo/client";

// Profile/hierarchy changes can alter these calculated memberships without
// changing the native Image or Scene relationships returned by the mutation.
export function evictEffectiveMediaAssociations(
  cache: Pick<ApolloCache<unknown>, "extract" | "evict">
) {
  const records = cache.extract() as Record<string, { __typename?: string }>;
  for (const [id, record] of Object.entries(records)) {
    if (record.__typename === "Image" || record.__typename === "Scene") {
      cache.evict({ id, fieldName: "effective_associations" });
    }
  }
}
