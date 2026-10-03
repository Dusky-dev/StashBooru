import assert from "node:assert/strict";
import { test } from "node:test";
import {
  nextStackMember,
  parseMemberURL,
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
