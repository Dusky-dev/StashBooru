import assert from "node:assert/strict";
import test from "node:test";

import {
  bulkSelectedImageIDs,
  bulkTaggingRequest,
  defaultBulkTaggingCategories,
  defaultBulkTaggingSources,
  MANUAL_BULK_WARNING_KEY,
  manualBulkWarningDismissed,
} from "../src/components/Tagging/bulkTaggingPolicy.ts";

test("bulk tagging is only offered for two or more selected Images", () => {
  assert.equal(
    bulkSelectedImageIDs([{ kind: "IMAGE", id: "1" }], "IMAGE"),
    undefined
  );
  assert.equal(
    bulkSelectedImageIDs(
      [
        { kind: "IMAGE", id: "1" },
        { kind: "VIDEO", id: "2" },
      ],
      "IMAGE"
    ),
    undefined
  );
  assert.deepEqual(
    bulkSelectedImageIDs(
      [
        { kind: "IMAGE", id: "1" },
        { kind: "IMAGE", id: "1200" },
      ],
      "IMAGE"
    ),
    [1, 1200]
  );
  assert.equal(
    bulkSelectedImageIDs(
      [
        { kind: "IMAGE", id: "123oops" },
        { kind: "IMAGE", id: "2" },
      ],
      "IMAGE"
    ),
    undefined
  );
  assert.equal(
    bulkSelectedImageIDs(
      [
        { kind: "IMAGE", id: "9007199254740992" },
        { kind: "IMAGE", id: "2" },
      ],
      "IMAGE"
    ),
    undefined
  );
});

test("bulk request is scoped to selected IDs and never replaces Artists", () => {
  assert.deepEqual(
    bulkTaggingRequest(
      [9, 10],
      {
        bulkTaggingEnabled: true,
        threshold: 0.492,
        eva02Threshold: 0.35,
        limit: 50,
      },
      { ...defaultBulkTaggingCategories, applyArtist: false },
      { ...defaultBulkTaggingSources, useCamie: false }
    ),
    {
      imageIDs: [9, 10],
      all: false,
      confirmed: true,
      threshold: 0.492,
      eva02Threshold: 0.35,
      useCamie: false,
      useEva02: true,
      limit: 50,
      applyCharacters: true,
      applyArtist: false,
      applyCopyright: true,
      applyTags: true,
      replaceArtist: false,
    }
  );
});

test("manual bulk warning is dismissible per browser and fails safe", () => {
  assert.equal(manualBulkWarningDismissed(), false);
  assert.equal(
    manualBulkWarningDismissed({
      getItem: (key) => (key === MANUAL_BULK_WARNING_KEY ? "1" : null),
    }),
    true
  );
  assert.equal(
    manualBulkWarningDismissed({
      getItem: () => {
        throw new Error("storage blocked");
      },
    }),
    false
  );
});
