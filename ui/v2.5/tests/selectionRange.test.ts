import assert from "node:assert/strict";
import test from "node:test";
import { selectionRange } from "../src/components/List/selectionRange.ts";

const page = [{ id: "image:2" }, { id: "scene:1" }, { id: "image:3" }];

test("a range with an off-page anchor includes the first visible item", () => {
  assert.deepEqual(
    selectionRange(page, "image:1", "scene:1"),
    page.slice(0, 2)
  );
  assert.deepEqual(selectionRange(page, "scene:9", "image:2"), [page[0]]);
});

test("a range with no anchor starts at the first visible item", () => {
  assert.deepEqual(selectionRange(page, undefined, "image:3"), page);
});

test("same-page ranges include both ends in either direction", () => {
  assert.deepEqual(selectionRange(page, "image:2", "image:3"), page);
  assert.deepEqual(selectionRange(page, "image:3", "image:2"), page);
  assert.deepEqual(selectionRange(page, "scene:1", "scene:1"), [page[1]]);
});

test("typed identities remain distinct when numeric IDs match", () => {
  const mixed = [{ id: "image:1" }, { id: "scene:1" }, { id: "image:2" }];
  assert.deepEqual(selectionRange(mixed, "scene:1", "image:2"), mixed.slice(1));
});

test("a stale target and an empty page add no items", () => {
  assert.deepEqual(selectionRange(page, "image:2", "image:9"), []);
  assert.deepEqual(selectionRange(page, undefined, "image:9"), []);
  assert.deepEqual(selectionRange([], "image:2", "image:3"), []);
});
