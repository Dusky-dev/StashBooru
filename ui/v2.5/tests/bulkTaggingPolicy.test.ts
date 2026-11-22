import assert from "node:assert/strict";
import test from "node:test";

import {
  availableBulkTaggingSources,
  bulkSelectedImageIDs,
  bulkTaggingRequest,
  defaultBulkTaggingCategories,
  taggingActionKind,
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
        filenameEnabled: true,
        threshold: 0.492,
        eva02Threshold: 0.35,
        limit: 50,
      },
      { ...defaultBulkTaggingCategories, applyArtist: false },
      { useFilename: true, useCamie: false, useEva02: true }
    ),
    {
      imageIDs: [9, 10],
      all: false,
      confirmed: true,
      threshold: 0.492,
      eva02Threshold: 0.35,
      useFilename: true,
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

test("local filename metadata can tag a selection without the model opt-in", () => {
  const config = {
    bulkTaggingEnabled: false,
    filenameEnabled: true,
    threshold: 0.492,
    eva02Threshold: 0.35,
    limit: 50,
  };
  assert.deepEqual(availableBulkTaggingSources(config), {
    useFilename: true,
    useCamie: false,
    useEva02: false,
  });
  // A stale model choice is never sent while the model opt-in is off.
  const request = bulkTaggingRequest(
    [1, 2],
    config,
    defaultBulkTaggingCategories,
    {
      useFilename: true,
      useCamie: true,
      useEva02: true,
    }
  );
  assert.equal(request.useFilename, true);
  assert.equal(request.useCamie, false);
  assert.equal(request.useEva02, false);
  assert.deepEqual(
    availableBulkTaggingSources({ ...config, filenameEnabled: false }),
    { useFilename: false, useCamie: false, useEva02: false }
  );
});

test("the tagging action is named after the selected media", () => {
  const image = { kind: "IMAGE", id: "1" };
  const video = { kind: "VIDEO", id: "1" };
  assert.equal(taggingActionKind([image], "IMAGE"), "image");
  assert.equal(taggingActionKind([video], "IMAGE"), "video");
  assert.equal(taggingActionKind([image, video], "IMAGE"), "mixed");
});
