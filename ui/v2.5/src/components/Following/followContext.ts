import type { BooruSource } from "../BooruBrowser/browser";
import type { ProviderIdentity } from "../Identities/identityApi";

// Sites whose feeds can be checked; keep in step with booru.FollowSyncEngine.
const syncEngines = ["danbooru", "gelbooru", "shimmie", "pawchive"];

export function canSyncEngine(engine: string | undefined) {
  return !!engine && syncEngines.includes(engine);
}

/** Whether a site name can be followed: a tag, or a Pawchive creator. */
export function canFollowTarget(
  engine: string | undefined,
  kind: string,
  target: string
) {
  if (!["artist", "character", "copyright"].includes(kind)) return false;
  // Pawchive follows a creator account (creator:service:id); others a tag.
  if (engine === "pawchive")
    return kind === "artist" && target.startsWith("creator:");
  // Same rule as the server's FollowQuery: one exact tag, no operators.
  return (
    canSyncEngine(engine) &&
    target.trim() !== "" &&
    !/[\s:*?{}|"\\]/.test(target) &&
    !/^[-~]/.test(target)
  );
}

export function canFollowIdentity(identity: ProviderIdentity) {
  return (
    canFollowTarget(identity.engine, identity.kind, identity.target) &&
    identity.form === (identity.engine === "pawchive" ? "account" : "tag")
  );
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
