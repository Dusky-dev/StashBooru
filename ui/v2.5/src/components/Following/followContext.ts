import type { BooruSource } from "../BooruBrowser/browser";
import type { ProviderIdentity } from "../Identities/identityApi";

export function canFollowIdentity(identity: ProviderIdentity) {
  return (
    identity.engine === "danbooru" &&
    identity.form === "tag" &&
    ["artist", "character", "copyright"].includes(identity.kind)
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
