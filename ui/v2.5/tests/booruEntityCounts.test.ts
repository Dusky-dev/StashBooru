import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import { parse, Kind, type SelectionNode } from "graphql";
import { booruEntityMediaCounts } from "../src/components/Shared/booruEntityCounts.ts";

test("entity count presentation keeps zero distinct from absent GraphQL fields", () => {
  assert.deepEqual(booruEntityMediaCounts({ scene_count: 7, image_count: 13 }), {
    videos: 7,
    images: 13,
  });
  assert.deepEqual(booruEntityMediaCounts({ scene_count: 0, image_count: 0 }), {
    videos: 0,
    images: 0,
  });
  assert.deepEqual(booruEntityMediaCounts({}), {
    videos: null,
    images: null,
  });
  assert.deepEqual(
    booruEntityMediaCounts({ scene_count: null, image_count: -1 }),
    {
      videos: null,
      images: null,
    }
  );
});

function selectionAt(selections: readonly SelectionNode[], name: string) {
  const field = selections.find(
    (selection) =>
      selection.kind === Kind.FIELD && selection.name.value === name
  );
  assert.ok(
    field && field.kind === Kind.FIELD && field.selectionSet,
    `Missing ${name}`
  );
  return field.selectionSet.selections;
}

function assertCounts(selections: readonly SelectionNode[], label: string) {
  for (const name of ["scene_count", "image_count"]) {
    assert.ok(
      selections.some(
        (selection) =>
          selection.kind === Kind.FIELD && selection.name.value === name
      ),
      `${label} missing ${name}`
    );
  }
}

for (const [file, fragment] of [
  ["image.graphql", "ImageData"],
  ["scene.graphql", "SceneEffectiveAssociations"],
]) {
  test(
    `${fragment} loads Artist and Copyright Image/Video counts, including hierarchy`,
    () => {
      const source = fs.readFileSync(
        new URL(`../graphql/data/${file}`, import.meta.url),
        "utf8"
      );
      const document = parse(source);
      const definition = document.definitions.find(
        (item) =>
          item.kind === Kind.FRAGMENT_DEFINITION && item.name.value === fragment
      );
      assert.ok(definition && definition.kind === Kind.FRAGMENT_DEFINITION);

      const effective = selectionAt(
        definition.selectionSet.selections,
        "effective_associations"
      );
      const artists = selectionAt(effective, "artists");
      const copyrights = selectionAt(effective, "copyrights");
      assertCounts(artists, `${fragment}.artists`);
      assertCounts(copyrights, `${fragment}.copyrights`);
      assertCounts(
        selectionAt(copyrights, "breadcrumb"),
        `${fragment}.copyrights.breadcrumb`
      );
    }
  );
}
