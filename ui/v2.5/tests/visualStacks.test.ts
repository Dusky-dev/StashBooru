import assert from "node:assert/strict";
import { test } from "node:test";
import {
  nextStackMember,
  parseMemberURL,
  reorderStackMembers,
  stackKey,
  stackPath,
} from "../src/components/VisualStacks/identity.ts";

test("stack identity and native URLs separate equal Image and Video IDs", () => {
  const image = parseMemberURL("https://stash.example/images/123");
  const video = parseMemberURL("/scenes/123?queue=ignored");
  assert.ok(image);
  assert.ok(video);
  assert.notEqual(stackKey(image), stackKey(video));
  assert.equal(stackPath(image), "/images/123");
  assert.equal(stackPath(video), "/scenes/123");
  for (const value of [
    "/images/0",
    "/images/-1",
    "/images/1/more",
    "javascript:alert(1)",
    "/scenes/999999999999999999999",
    "/performers/123",
  ])
    assert.equal(parseMemberURL(value), undefined);
});

test("reordering moves typed identities in either direction without changing labels or representative", () => {
  const rows = [
    { id: "image:1", label: "Original", representative: true },
    { id: "scene:1", label: "Converted copy", representative: false },
    { id: "image:2", label: "Restoration", representative: false },
  ];
  const down = reorderStackMembers(rows, "image:1", "image:2");
  assert.deepEqual(
    down.map((row) => row.id),
    ["scene:1", "image:2", "image:1"]
  );
  assert.equal(down[2], rows[0]);
  assert.deepEqual(reorderStackMembers(down, "image:1", "scene:1"), rows);
  assert.equal(reorderStackMembers(rows, "unknown", "image:1"), rows);
  assert.equal(reorderStackMembers(rows, "image:1", "unknown"), rows);
  assert.equal(reorderStackMembers(rows, "scene:1", "scene:1"), rows);
});

test("variant navigation follows explicit order and retains selected identity across reorders", () => {
  const ordered = ["image:1", "scene:1", "image:2"];
  assert.equal(nextStackMember(ordered, "scene:1", 1), "image:2");
  assert.equal(nextStackMember(ordered, "image:1", -1), "image:2");
  assert.equal(
    nextStackMember(["image:2", "scene:1", "image:1"], "scene:1", 1),
    "image:1"
  );
  assert.equal(nextStackMember([], "scene:1", 1), undefined);
});
