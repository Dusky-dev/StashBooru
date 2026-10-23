import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
import { evictEffectiveMediaAssociations } from "../src/core/mediaCache.ts";

// Apollo's CommonJS entry works with the repository's Node test loader.
const { InMemoryCache, gql } = createRequire(import.meta.url)(
  "@apollo/client/core"
);
const query = gql`
  query CachedMedia {
    image: findImage(id: "1") { id title tags { id name } effective_associations { tags { id name } } }
    scene: findScene(id: "1") { id title tags { id name } effective_associations { tags { id name } } }
    studio: findStudio(id: "1") { id name }
  }
`;
function fixture() {
  const cache = new InMemoryCache();
  const direct = { __typename: "Tag", id: "1", name: "direct" };
  const inherited = { __typename: "Tag", id: "2", name: "inherited" };
  const media = (kind: string) => ({
    __typename: kind,
    id: "1",
    title: kind,
    tags: [direct],
    effective_associations: {
      __typename: "MediaEffectiveAssociations",
      tags: [direct, inherited],
    },
  });
  cache.writeQuery({
    query,
    data: {
      image: media("Image"),
      scene: media("Scene"),
      studio: { __typename: "Studio", id: "1", name: "Artist" },
    },
  });
  return cache;
}

test("derived memberships are invalidated for both media types with equal IDs", () => {
  const cache = fixture();
  assert.equal(cache.diff({ query }).complete, true);
  evictEffectiveMediaAssociations(cache);
  const records = cache.extract();
  for (const key of ["Image:1", "Scene:1"]) {
    assert.equal("effective_associations" in records[key], false);
    assert.deepEqual(records[key].tags, [{ __ref: "Tag:1" }]);
    assert.equal(records[key].id, "1");
  }
  assert.equal(
    cache.diff({ query }).complete,
    false,
    "A cached detail read must request fresh calculated memberships"
  );
});

test("direct metadata, source records and unrelated profiles survive invalidation", () => {
  const cache = fixture();
  const before = cache.extract();
  evictEffectiveMediaAssociations(cache);
  const after = cache.extract();
  for (const key of ["Tag:1", "Tag:2", "Studio:1", "ROOT_QUERY"])
    assert.deepEqual(after[key], before[key]);
  for (const key of ["Image:1", "Scene:1"]) {
    const { effective_associations: _derived, ...native } = before[key];
    assert.deepEqual({ ...after[key] }, native);
  }
});

test("empty and repeated invalidations leave the cache consistent", () => {
  const empty = new InMemoryCache();
  evictEffectiveMediaAssociations(empty);
  assert.deepEqual(empty.extract(), {});
  const cache = fixture();
  evictEffectiveMediaAssociations(cache);
  const once = cache.extract();
  evictEffectiveMediaAssociations(cache);
  assert.deepEqual(cache.extract(), once);
});
