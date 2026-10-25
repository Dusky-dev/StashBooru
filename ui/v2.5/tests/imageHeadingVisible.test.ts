import assert from "node:assert/strict";
import test from "node:test";

import { imageHeadingVisible } from "../src/utils/imageHeadingVisible.ts";

test("image filename headings are visible by default", () => {
  assert.equal(imageHeadingVisible({ title: null }), true);
  assert.equal(imageHeadingVisible({}), true);
  assert.equal(imageHeadingVisible({ title: "" }), true);
});

test("hide filename fallback but keep explicit image titles", () => {
  assert.equal(imageHeadingVisible({ title: null }, true), false);
  assert.equal(imageHeadingVisible({ title: "" }, true), false);
  assert.equal(imageHeadingVisible({}, true), false);
  assert.equal(imageHeadingVisible({ title: "Holiday photo" }, true), true);
  assert.equal(imageHeadingVisible({ title: "custom.jpg" }, true), true);
  assert.equal(imageHeadingVisible({ title: "Holiday photo" }, false), true);
});
