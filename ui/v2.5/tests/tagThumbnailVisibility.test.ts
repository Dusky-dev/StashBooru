import assert from "node:assert/strict";
import test from "node:test";

import { shouldHideTagThumbnail } from "../src/components/Tags/tagThumbnailVisibility.ts";

test("general Tag thumbnails remain visible by default", () => {
  assert.equal(shouldHideTagThumbnail(false, false, false), false);
});

test("global setting hides general Tag thumbnails", () => {
  assert.equal(shouldHideTagThumbnail(false, true, false), true);
});

test("Copyright thumbnail is unaffected by general Tag preference", () => {
  assert.equal(shouldHideTagThumbnail(false, true, true), false);
});

test("per-tag choice hides thumbnails even for Copyright tags", () => {
  assert.equal(shouldHideTagThumbnail(true, false, false), true);
  assert.equal(shouldHideTagThumbnail(true, true, false), true);
  assert.equal(shouldHideTagThumbnail(true, true, true), true);
});
