import type { BooruSource } from "../BooruBrowser/browser";
import type { ProviderIdentity } from "../Identities/identityApi";

// Sites whose feeds can be checked; keep in step with booru.FollowSyncEngine.
const syncEngines = ["danbooru", "gelbooru", "shimmie", "pawchive"];

export function canSyncEngine(engine: string | undefined) {
  return !!engine && syncEngines.includes(engine);
}

export function canFollowIdentity(identity: ProviderIdentity) {
  if (!["artist", "character", "copyright"].includes(identity.kind))
    return false;
  // Pawchive follows a creator account (creator:service:id); others a tag.
  if (identity.engine === "pawchive")
    return (
      identity.kind === "artist" &&
      identity.form === "account" &&
      identity.target.startsWith("creator:")
    );
  return canSyncEngine(identity.engine) && identity.form === "tag";
}

export function sourceForIdentity(
  sources: BooruSource[],
  identity: ProviderIdentity
) {
  if (!canFollowIdentity(identity)) return undefined;
  const matches = sources.filter(
    (source) =>
      source.engine === identity.engine && source.instance === identity.instance
  );
  return (
    matches.find((source) => source.id === identity.sourceID) ?? matches[0]
  );
}
