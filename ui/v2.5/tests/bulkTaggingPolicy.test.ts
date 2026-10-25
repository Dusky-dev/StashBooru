import assert from "node:assert/strict";
import test from "node:test";

import {
  bulkSelectedImageIDs,
  bulkTaggingRequest,
  defaultBulkTaggingCategories,
} from "../src/components/Tagging/bulkTaggingPolicy.ts";

test("bulk tagging is only offered for two or more selected Images", () => {
  assert.equal(
    bulkSelectedImageIDs([{ kind: "IMAGE", id: "1" }], "IMAGE"),
    undefined
  );
  assert.equal(
    bulkSelectedImageIDs(
      [{ kind: "IMAGE", id: "1" }, { kind: "VIDEO", id: "2" }],
      "IMAGE"
    ),
    undefined
  );
  assert.deepEqual(
    bulkSelectedImageIDs(
      [{ kind: "IMAGE", id: "1" }, { kind: "IMAGE", id: "1200" }],
      "IMAGE"
    ),
    [1, 1200]
  );
  assert.equal(
    bulkSelectedImageIDs(
      [{ kind: "IMAGE", id: "123oops" }, { kind: "IMAGE", id: "2" }],
      "IMAGE"
    ),
    undefined
  );
  assert.equal(
    bulkSelectedImageIDs(
      [{ kind: "IMAGE", id: "9007199254740992" }, { kind: "IMAGE", id: "2" }],
      "IMAGE"
    ),
    undefined
  );
});

test("bulk request is scoped to selected IDs and never replaces Artists", () => {
  assert.deepEqual(
    bulkTaggingRequest(
      [9, 10],
      { bulkTaggingEnabled: true, threshold: 0.492, limit: 50 },
      { ...defaultBulkTaggingCategories, applyArtist: false }
    ),
    {
      imageIDs: [9, 10],
      all: false,
      threshold: 0.492,
      limit: 50,
      applyCharacters: true,
      applyArtist: false,
      applyCopyright: true,
      applyTags: true,
      replaceArtist: false,
    }
  );
});
